package json

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/cybergodev/json/internal"
)

// ============================================================================
// ENCODING LOW-COVERAGE TESTS
// Target: parseBoolean, parseNull, Encode, EncodePretty, encodeJSONNumber,
//         validateTimeFormat, encodeStruct, validateDepth
// ============================================================================

// TestDecoderParseBoolean tests parseBoolean via Decoder
func TestDecoderParseBoolean(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
		wantErr  bool
	}{
		{"True", `true`, true, false},
		{"False", `false`, false, false},
		{"InvalidTrue", `trx`, false, true},
		{"InvalidFalse", `fals`, false, true},
		{"TrueInArray", `[true]`, true, false},
		{"FalseInObject", `{"val":false}`, false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dec := NewDecoder(strings.NewReader(tt.input))
			var result any
			err := dec.Decode(&result)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			// For array/object, extract the boolean
			switch v := result.(type) {
			case bool:
				if v != tt.expected {
					t.Errorf("got %v, want %v", v, tt.expected)
				}
			case []any:
				if len(v) > 0 {
					if b, ok := v[0].(bool); ok && b != tt.expected {
						t.Errorf("got %v, want %v", b, tt.expected)
					}
				}
			case map[string]any:
				if val, ok := v["val"]; ok {
					if b, ok := val.(bool); ok && b != tt.expected {
						t.Errorf("got %v, want %v", b, tt.expected)
					}
				}
			}
		})
	}
}

// TestDecoderParseNull tests parseNull via Decoder
func TestDecoderParseNull(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"ValidNull", `null`, false},
		{"InvalidNull", `nul`, true},
		{"NullInArray", `[null]`, false},
		{"NullInObject", `{"val":null}`, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dec := NewDecoder(strings.NewReader(tt.input))
			var result any
			err := dec.Decode(&result)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

// TestProcessorEncodeMethods tests Encode and EncodePretty methods
func TestEncodeJSONNumber(t *testing.T) {
	tests := []struct {
		name            string
		num             json.Number
		preserveNumbers bool
		wantContains    string
	}{
		{"Integer", json.Number("42"), false, "42"},
		{"Float", json.Number("3.14"), false, "3.14"},
		{"Scientific", json.Number("1e10"), false, ""},
		{"PreservedInteger", json.Number("42"), true, "42"},
		{"PreservedFloat", json.Number("3.141592653589793"), true, "3.141592653589793"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := DefaultConfig()
			cfg.PreserveNumbers = tt.preserveNumbers

			data := map[string]any{"num": tt.num}
			result, err := Encode(data, cfg)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if tt.wantContains != "" && !strings.Contains(result, tt.wantContains) {
				t.Errorf("result %q should contain %q", result, tt.wantContains)
			}
		})
	}
}

// TestValidateDepth tests the depth validation
func TestValidateDepth(t *testing.T) {
	processor, err := New()
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer processor.Close()

	t.Run("WithinLimit", func(t *testing.T) {
		data := map[string]any{"a": map[string]any{"b": "value"}}
		err := processor.validateDepth(data, 10, 0)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("ExceedsLimit", func(t *testing.T) {
		// Create deeply nested structure
		deepData := map[string]any{"a": "value"}
		for i := 0; i < 20; i++ {
			deepData = map[string]any{"nested": deepData}
		}

		err := processor.validateDepth(deepData, 5, 0)
		if err == nil {
			t.Error("expected error for exceeding depth limit")
		}
	})

	t.Run("WithArray", func(t *testing.T) {
		data := []any{[]any{[]any{"deep"}}}
		err := processor.validateDepth(data, 10, 0)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("WithMapAnyKey", func(t *testing.T) {
		data := map[any]any{"key": "value"}
		err := processor.validateDepth(data, 10, 0)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
}

// TestEncodeStructCustom tests custom struct encoding paths
func TestEncodeStructCustom(t *testing.T) {
	type TestStruct struct {
		Name   string  `json:"name"`
		Value  int     `json:"value"`
		Hidden string  `json:"-"`
		Empty  *string `json:"empty,omitempty"`
	}

	tests := []struct {
		name     string
		config   Config
		input    TestStruct
		contains string
		omit     string
	}{
		{
			name:     "Default",
			config:   DefaultConfig(),
			input:    TestStruct{Name: "test", Value: 42, Hidden: "secret"},
			contains: `"name"`,
		},
		{
			name:     "SortKeys",
			config:   func() Config { c := DefaultConfig(); c.SortKeys = true; return c }(),
			input:    TestStruct{Name: "test", Value: 42},
			contains: `"name"`,
		},
		{
			name:     "NoEscapeHTML",
			config:   func() Config { c := DefaultConfig(); c.EscapeHTML = false; return c }(),
			input:    TestStruct{Name: "<script>", Value: 1},
			contains: `"name"`,
		},
		{
			name:   "IncludeNullsFalse",
			config: func() Config { c := DefaultConfig(); c.IncludeNulls = false; return c }(),
			input:  TestStruct{Name: "test", Value: 42, Empty: nil},
			omit:   `"empty"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := Encode(tt.input, tt.config)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if tt.contains != "" && !strings.Contains(result, tt.contains) {
				t.Errorf("result %q should contain %q", result, tt.contains)
			}

			if tt.omit != "" && strings.Contains(result, tt.omit) {
				t.Errorf("result %q should not contain %q", result, tt.omit)
			}

			// Hidden field should never appear
			if strings.Contains(result, "secret") {
				t.Error("result should not contain hidden field value")
			}
		})
	}
}

// TestValidateFormats tests all format validation functions via table-driven subtests.
func TestValidateFormats(t *testing.T) {
	processor, err := New()
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer processor.Close()

	type validateFunc func(string, string, *[]ValidationError) error

	tests := []struct {
		name    string
		fn      validateFunc
		input   string
		wantErr bool
	}{
		// Time
		{"Time/Valid", processor.validateTimeFormat, "12:30:45", false},
		{"Time/InvalidHour", processor.validateTimeFormat, "25:00:00", true},
		{"Time/InvalidFormat", processor.validateTimeFormat, "12-30-45", true},
		{"Time/Partial", processor.validateTimeFormat, "12:30", true},
		// DateTime
		{"DateTime/Valid", processor.validateDateTimeFormat, "2024-01-15T10:30:00Z", false},
		{"DateTime/WithOffset", processor.validateDateTimeFormat, "2024-01-15T10:30:00+07:00", false},
		{"DateTime/Invalid", processor.validateDateTimeFormat, "2024-13-45T99:99:99Z", true},
		{"DateTime/InvalidFormat", processor.validateDateTimeFormat, "2024/01/15 10:30:00", true},
		// Email
		{"Email/Valid", processor.validateEmailFormat, "test@example.com", false},
		{"Email/Invalid", processor.validateEmailFormat, "not-an-email", true},
		{"Email/Empty", processor.validateEmailFormat, "", true},
		// URI
		{"URI/ValidHTTPS", processor.validateURIFormat, "https://example.com", false},
		{"URI/ValidFTP", processor.validateURIFormat, "ftp://files.example.com", false},
		{"URI/Invalid", processor.validateURIFormat, "not-a-uri", true},
		{"URI/Empty", processor.validateURIFormat, "", true},
		// UUID
		{"UUID/Valid", processor.validateUUIDFormat, "550e8400-e29b-41d4-a716-446655440000", false},
		{"UUID/Invalid", processor.validateUUIDFormat, "not-a-uuid", true},
		{"UUID/Empty", processor.validateUUIDFormat, "", true},
		// IPv4
		{"IPv4/Valid", processor.validateIPv4Format, "192.168.1.1", false},
		{"IPv4/Valid2", processor.validateIPv4Format, "10.0.0.1", false},
		{"IPv4/TooManyParts", processor.validateIPv4Format, "192.168.1.1.1", true},
		{"IPv4/TooFewParts", processor.validateIPv4Format, "192.168.1", true},
		{"IPv4/OutOfRange", processor.validateIPv4Format, "192.168.1.256", true},
		{"IPv4/NotNumber", processor.validateIPv4Format, "192.168.1.abc", true},
		{"IPv4/Negative", processor.validateIPv4Format, "192.168.1.-1", true},
		// IPv6
		{"IPv6/Valid", processor.validateIPv6Format, "2001:0db8:85a3:0000:0000:8a2e:0370:7334", false},
		{"IPv6/Short", processor.validateIPv6Format, "::1", false},
		{"IPv6/NoColon", processor.validateIPv6Format, "192.168.1.1", true},
		{"IPv6/Empty", processor.validateIPv6Format, "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var errs []ValidationError
			tt.fn(tt.input, "test.path", &errs)

			if tt.wantErr && len(errs) == 0 {
				t.Error("expected validation error")
			}
			if !tt.wantErr && len(errs) > 0 {
				t.Errorf("unexpected validation errors: %v", errs)
			}
		})
	}
}

// ============================================================================
// CONSOLIDATED LOW-COVERAGE TESTS
// Target: maybeEvictConfigCache (16%), getProcessorWithConfig (50%),
// Clone (57%), withProcessor (60%)
// ============================================================================

// testValidatorImpl is a simple Validator implementation for testing
type testValidatorImpl struct {
	validateCalled bool
}

func (v *testValidatorImpl) Validate(jsonStr string) error {
	v.validateCalled = true
	return nil
}

// TestMaybeEvictConfigCache tests the cache eviction logic
func TestMaybeEvictConfigCache(t *testing.T) {
	// Clear cache before testing
	configProcessorCacheMu.Lock()
	configProcessorCache.Range(func(key, value any) bool {
		configProcessorCache.Delete(key)
		return true
	})
	configProcessorCacheMu.Unlock()

	t.Run("NoEvictionWhenBelowLimit", func(t *testing.T) {
		// Create a few processors (below limit)
		for i := 0; i < 3; i++ {
			cfg := DefaultConfig()
			cfg.MaxCacheSize = 50 + i // Unique config
			_, err := getProcessorWithConfig(cfg)
			if err != nil {
				t.Fatalf("getProcessorWithConfig error: %v", err)
			}
		}
		// Verify cache has entries
		var count int
		configProcessorCache.Range(func(_, _ any) bool {
			count++
			return true
		})
		if count < 3 {
			t.Errorf("Expected at least 3 cache entries, got %d", count)
		}
	})

	t.Run("EvictionWhenOverLimit", func(t *testing.T) {
		// Fill cache to trigger eviction
		for i := 0; i < 110; i++ {
			cfg := DefaultConfig()
			cfg.MaxCacheSize = 1000 + i // Unique config
			_, err := getProcessorWithConfig(cfg)
			if err != nil {
				t.Fatalf("getProcessorWithConfig error at %d: %v", i, err)
			}
		}

		// Verify cache was evicted (should be below limit now)
		var count int
		configProcessorCache.Range(func(_, _ any) bool {
			count++
			return true
		})

		if count > configProcessorCacheLimit {
			t.Errorf("Cache count %d exceeds limit %d after eviction", count, configProcessorCacheLimit)
		}
	})

	t.Run("EvictClosedProcessors", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.MaxCacheSize = 9999 // Unique
		p, err := getProcessorWithConfig(cfg)
		if err != nil {
			t.Fatalf("getProcessorWithConfig error: %v", err)
		}
		p.Close()

		// Fill cache more to trigger eviction which should remove closed processors
		for i := 0; i < 100; i++ {
			cfg := DefaultConfig()
			cfg.MaxCacheSize = 2000 + i // Unique
			_, err := getProcessorWithConfig(cfg)
			if err != nil {
				t.Fatalf("getProcessorWithConfig error at %d: %v", i, err)
			}
		}
	})
}

// TestGetProcessorWithConfig tests the config-based processor retrieval
func TestGetProcessorWithConfig(t *testing.T) {
	t.Run("ReturnsCachedProcessor", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.MaxCacheSize = 5000 // Unique config

		p1, err := getProcessorWithConfig(cfg)
		if err != nil {
			t.Fatalf("First call error: %v", err)
		}

		p2, err := getProcessorWithConfig(cfg)
		if err != nil {
			t.Fatalf("Second call error: %v", err)
		}

		if p1 != p2 {
			t.Error("Expected cached processor to be returned")
		}
	})

	t.Run("DifferentConfigReturnsDifferentProcessor", func(t *testing.T) {
		cfg1 := DefaultConfig()
		cfg1.MaxCacheSize = 6001

		cfg2 := DefaultConfig()
		cfg2.MaxCacheSize = 6002

		p1, err := getProcessorWithConfig(cfg1)
		if err != nil {
			t.Fatalf("First call error: %v", err)
		}

		p2, err := getProcessorWithConfig(cfg2)
		if err != nil {
			t.Fatalf("Second call error: %v", err)
		}

		if p1 == p2 {
			t.Error("Expected different processors for different configs")
		}
	})

	t.Run("ConcurrentAccess", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.MaxCacheSize = 8001

		var wg sync.WaitGroup
		var processors []*Processor
		var mu sync.Mutex

		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				p, err := getProcessorWithConfig(cfg)
				if err != nil {
					t.Errorf("Concurrent call error: %v", err)
					return
				}
				mu.Lock()
				processors = append(processors, p)
				mu.Unlock()
			}()
		}
		wg.Wait()

		for i := 1; i < len(processors); i++ {
			if processors[i] != processors[0] {
				t.Error("Expected all concurrent calls to return same cached processor")
				break
			}
		}
	})
}

// TestWithProcessorErrorPaths tests error handling paths
func TestWithProcessorErrorPaths(t *testing.T) {
	t.Run("ConfigValidation clamps invalid values", func(t *testing.T) {
		cfg := Config{MaxJSONSize: -1}
		// Validate mutates in place, clamping invalid values, and returns nil.
		if err := cfg.Validate(); err != nil {
			t.Fatalf("Validate returned unexpected error: %v", err)
		}
		if cfg.MaxJSONSize <= 0 {
			t.Errorf("MaxJSONSize not clamped to a valid minimum, got %d", cfg.MaxJSONSize)
		}

		// ValidateWithWarnings reports the correction Validate applied silently.
		cfg2 := Config{MaxJSONSize: -1}
		if warnings := cfg2.ValidateWithWarnings(); len(warnings) == 0 {
			t.Error("expected at least one warning for MaxJSONSize=-1")
		}
	})
}

// TestGetTypedEdgeCases tests GetTyped edge cases
func TestGetTypedEdgeCases(t *testing.T) {
	tests := []struct {
		name    string
		jsonStr string
		path    string
		defVal  string
		wantVal string
	}{
		{"InvalidJSON", `{invalid}`, "key", "default", "default"},
		{"PathNotFound", `{"key":"value"}`, "missing", "default", "default"},
		{"IntConvertedToString", `{"key":123}`, "key", "default", "123"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetTyped(tt.jsonStr, tt.path, tt.defVal)
			if result != tt.wantVal {
				t.Errorf("GetTyped() = %q, want %q", result, tt.wantVal)
			}
		})
	}
}

// TestParseEdgeCases tests Parse function edge cases
func TestParseEdgeCases(t *testing.T) {
	t.Run("ParseWithConfig", func(t *testing.T) {
		cfg := Config{MaxJSONSize: -1}
		result, err := ParseAny(`{"key":"value"}`, cfg)
		if err != nil {
			t.Fatalf("ParseAny with auto-corrected config failed: %v", err)
		}
		if result == nil {
			t.Error("ParseAny should return a parsed value")
		}
	})

	t.Run("ParseEmptyString", func(t *testing.T) {
		result, err := ParseAny(``)
		if err == nil {
			t.Errorf("Parse empty string should fail: %v", err)
		}
		_ = result
	})

	t.Run("ParseArray", func(t *testing.T) {
		result, err := ParseAny(`[1, 2, 3]`)
		if err != nil {
			t.Errorf("Parse array failed: %v", err)
		}
		arr, ok := result.([]any)
		if !ok {
			t.Errorf("Expected []any, got %T", result)
		}
		if len(arr) != 3 {
			t.Errorf("Expected 3 elements, got %d", len(arr))
		}
	})
}

// TestValidEdgeCases tests Valid function edge cases
func TestValidEdgeCases(t *testing.T) {
	t.Run("ValidEmptyBytes", func(t *testing.T) {
		if Valid([]byte{}) {
			t.Error("Valid([]byte{}) should return false")
		}
	})

	t.Run("ValidNilBytes", func(t *testing.T) {
		if Valid(nil) {
			t.Error("Valid(nil) should return false")
		}
	})
}

// TestFormatJSONStringEdgeCases tests Processor.formatJSONString edge cases
func TestFormatJSONStringEdgeCases(t *testing.T) {
	p, _ := New()
	defer p.Close()

	t.Run("InvalidJSONIsQuotedAsScalarString", func(t *testing.T) {
		// formatJSONString treats non-JSON input as a bare string scalar and
		// quotes it rather than erroring.
		result, err := p.formatJSONString("{invalid}", false)
		if err != nil {
			t.Fatalf("formatJSONString on invalid JSON: %v", err)
		}
		if result != "\"{invalid}\"" {
			t.Errorf("formatJSONString on invalid JSON = %q, want quoted input", result)
		}
	})

	t.Run("ValidJSON", func(t *testing.T) {
		result, err := p.formatJSONString(`{"key":"value"}`, false)
		if err != nil {
			t.Errorf("Processor.formatJSONString failed: %v", err)
		}
		if result == "" {
			t.Error("Processor.formatJSONString should return non-empty result")
		}
	})
}

// ============================================================================
// HELPER FUNCTION TESTS
// ============================================================================

// TestHelperFunctions tests various helper functions
func TestHelperFunctions(t *testing.T) {
	t.Run("isValidJSON", func(t *testing.T) {
		if !isValidJSON(`{"key":"value"}`) {
			t.Error("isValidJSON should return true for valid JSON")
		}
		if isValidJSON(`{invalid}`) {
			t.Error("isValidJSON should return false for invalid JSON")
		}
	})

	t.Run("validatePath", func(t *testing.T) {
		if err := internal.ValidatePath("key.nested"); err != nil {
			t.Errorf("validatePath should return nil for valid path: %v", err)
		}
	})

	t.Run("CompareJSON", func(t *testing.T) {
		equal, err := CompareJSON(`{"a":1}`, `{"a":1}`)
		if err != nil || !equal {
			t.Errorf("CompareJSON should return true for equal JSON: %v, %v", equal, err)
		}
	})
}

// TestValidationChain tests validationChain
func TestValidationChain(t *testing.T) {
	t.Run("EmptyChain", func(t *testing.T) {
		chain := validationChain{}
		err := chain.Validate(`{"key":"value"}`)
		if err != nil {
			t.Errorf("Empty chain should pass: %v", err)
		}
	})

	t.Run("ChainWithValidators", func(t *testing.T) {
		chain := validationChain{&testValidatorImpl{}}
		err := chain.Validate(`{"key":"value"}`)
		if err != nil {
			t.Errorf("Chain should pass: %v", err)
		}
	})
}

// TestHookFunc tests HookFunc
func TestHookFunc(t *testing.T) {
	t.Run("BeforeNil", func(t *testing.T) {
		h := &HookFunc{}
		err := h.Before(HookContext{})
		if err != nil {
			t.Errorf("Before with nil BeforeFn should return nil: %v", err)
		}
	})

	t.Run("AfterNil", func(t *testing.T) {
		h := &HookFunc{}
		result, err := h.After(HookContext{}, "test", nil)
		if err != nil || result != "test" {
			t.Errorf("After with nil AfterFn should return original: %v, %v", result, err)
		}
	})
}

// TestPatternLevel tests PatternLevel.String
func TestPatternLevel(t *testing.T) {
	tests := []struct {
		level    PatternLevel
		expected string
	}{
		{PatternLevelCritical, "critical"},
		{PatternLevelWarning, "warning"},
		{PatternLevelInfo, "info"},
		{PatternLevel(99), "unknown"},
	}

	for _, tt := range tests {
		result := tt.level.String()
		if result != tt.expected {
			t.Errorf("PatternLevel(%d).String() = %q, want %q", tt.level, result, tt.expected)
		}
	}
}

// TestNewSegmentFunctions tests segment creation functions
// TestNewSegmentFunctions covers the one segment constructor not already
// asserted by the internal package tests; the other subtests (Type-only
// checks duplicating internal tests) were removed in the FIX-001 cleanup.
func TestNewSegmentFunctions(t *testing.T) {
	t.Run("newAppendSegment", func(t *testing.T) {
		seg := newAppendSegment()
		if seg.Type != internal.AppendSegment {
			t.Error("newAppendSegment should create append segment")
		}
	})
}

// ============================================================================
// FILE LOW-COVERAGE TESTS
// Target: hasPrefixIgnoreCase, validateUnixPath, containsConsecutiveDots,
//         validatePathSymlinks, validatePathPlatform
// ============================================================================

// TestHasPrefixIgnoreCase tests the hasPrefixIgnoreCase function
func TestHasPrefixIgnoreCase(t *testing.T) {
	tests := []struct {
		s, prefix string
		expected  bool
	}{
		{"Hello", "He", true},
		{"Hello", "he", true},
		{"HELLO", "he", true},
		{"hello", "HE", true},
		{"Hello", "World", false},
		{"Hi", "Hello", false},
		{"", "", true},
		{"a", "", true},
		{"", "a", false},
		{"ABC", "abc", true},
		{"abc", "ABC", true},
		{"AbCdEf", "aBc", true},
		{"Test123", "TEST", true},
		{"Test123", "test", true},
		{"123Test", "123", true},
	}

	for _, tt := range tests {
		t.Run(tt.s+"_"+tt.prefix, func(t *testing.T) {
			result := hasPrefixIgnoreCase(tt.s, tt.prefix)
			if result != tt.expected {
				t.Errorf("hasPrefixIgnoreCase(%q, %q) = %v, want %v", tt.s, tt.prefix, result, tt.expected)
			}
		})
	}
}

// TestAPI_UnmarshalFromFile tests the top-level UnmarshalFromFile function
// Note: LoadFromFile, SaveToFile, MarshalToFile are tested in file_test.go
func TestAPI_UnmarshalFromFile(t *testing.T) {
	tests := []struct {
		name        string
		setupFile   bool
		fileContent string
		target      any
		wantErr     bool
		checkResult func(t *testing.T, result any)
	}{
		{
			name:        "UnmarshalFromFileValid",
			setupFile:   true,
			fileContent: `{"name":"test","value":123}`,
			target: &struct {
				Name  string
				Value int
			}{},
			wantErr: false,
			checkResult: func(t *testing.T, result any) {
				r := result.(*struct {
					Name  string
					Value int
				})
				if r.Name != "test" || r.Value != 123 {
					t.Errorf("Result = %+v, want {Name:test, Value:123}", r)
				}
			},
		},
		{
			name:      "UnmarshalFromFileNonExistent",
			setupFile: false,
			target:    &map[string]any{},
			wantErr:   true,
		},
		{
			name:        "UnmarshalFromFileNilTarget",
			setupFile:   true,
			fileContent: `{}`,
			target:      nil,
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var filePath string
			if tt.setupFile {
				tempDir := t.TempDir()
				filePath = filepath.Join(tempDir, "test.json")
				os.WriteFile(filePath, []byte(tt.fileContent), 0644)
			} else {
				filePath = "/non/existent/file.json"
			}

			err := UnmarshalFromFile(filePath, tt.target)
			if (err != nil) != tt.wantErr {
				t.Errorf("UnmarshalFromFile() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && tt.checkResult != nil {
				tt.checkResult(t, tt.target)
			}
		})
	}
}

// ============================================================================
// CONFIG TESTS - Coverage for Clone, Validate edge cases
// ============================================================================

// TestConfigCloneZero tests Config.Clone on zero value

// ============================================================================
// ENCODING TESTS - Coverage for printData branches
// ============================================================================

// TestCompactError tests Compact function error case
func TestCompactError(t *testing.T) {
	var dst bytes.Buffer
	err := Compact(&dst, []byte(`{invalid}`))
	if err == nil {
		t.Error("Compact should return error for invalid JSON")
	}
}

// TestIndentError tests Indent function error case
func TestIndentError(t *testing.T) {
	var dst bytes.Buffer
	err := Indent(&dst, []byte(`{invalid}`), "", "  ")
	if err == nil {
		t.Error("Indent should return error for invalid JSON")
	}
}

// TestEncodeWithConfig tests EncodeWithConfig with custom config
func TestEncodeWithConfig(t *testing.T) {
	t.Run("WithPretty", func(t *testing.T) {
		opts := DefaultConfig()
		opts.Pretty = true

		result, err := EncodeWithConfig(map[string]any{"key": "value"}, opts)
		if err != nil {
			t.Errorf("EncodeWithConfig failed: %v", err)
		}
		if !strings.Contains(result, "\n") {
			t.Error("Result should be pretty-printed")
		}
	})

	t.Run("WithCompact", func(t *testing.T) {
		opts := DefaultConfig()
		opts.Pretty = false

		result, err := EncodeWithConfig(map[string]any{"key": "value"}, opts)
		if err != nil {
			t.Errorf("EncodeWithConfig failed: %v", err)
		}
		if strings.Contains(result, "\n") {
			t.Error("Result should not be pretty-printed")
		}
	})
}

// TestEncode tests Encode function
func TestEncode(t *testing.T) {
	t.Run("WithoutConfig", func(t *testing.T) {
		result, err := Encode(map[string]any{"key": "value"})
		if err != nil {
			t.Errorf("Encode failed: %v", err)
		}
		if !strings.Contains(result, `"key"`) {
			t.Error("Result should contain key")
		}
	})
}

// ============================================================================
// API PRINT FUNCTIONS TESTS
// ============================================================================

// ============================================================================
// PROCESSOR METHODS TESTS - Additional coverage
// ============================================================================

// TestProcessorClosedState tests processor operations when closed
func TestProcessorClosedState(t *testing.T) {
	t.Run("ClosedProcessorOperations", func(t *testing.T) {
		processor, _ := New()
		processor.Close()

		// All operations should fail on closed processor
		_, err := processor.Get(`{"key":"value"}`, "key")
		if err == nil {
			t.Error("Get should fail on closed processor")
		}

		_, err = processor.Set(`{"key":"value"}`, "key", "new")
		if err == nil {
			t.Error("Set should fail on closed processor")
		}

		_, err = processor.Delete(`{"key":"value"}`, "key")
		if err == nil {
			t.Error("Delete should fail on closed processor")
		}

		_, err = processor.Marshal(map[string]any{"key": "value"})
		if err == nil {
			t.Error("Marshal should fail on closed processor")
		}

		err = processor.Unmarshal([]byte(`{"key":"value"}`), &map[string]any{})
		if err == nil {
			t.Error("Unmarshal should fail on closed processor")
		}
	})
}

// TestProcessorValidBytes tests ValidBytes method
func TestProcessorValidBytes(t *testing.T) {
	processor, _ := New()
	defer processor.Close()

	tests := []struct {
		input    []byte
		expected bool
	}{
		{[]byte(`{"key":"value"}`), true},
		{[]byte(`[1, 2, 3]`), true},
		{[]byte(`"string"`), true},
		{[]byte(`123`), true},
		{[]byte(`{invalid}`), false},
		{[]byte(``), false},
	}

	for _, tt := range tests {
		result := processor.ValidBytes(tt.input)
		if result != tt.expected {
			t.Errorf("ValidBytes(%q) = %v, want %v", tt.input, result, tt.expected)
		}
	}
}

// TestProcessorParse tests Parse method
func TestProcessorParse(t *testing.T) {
	processor, _ := New()
	defer processor.Close()

	t.Run("ParseToMap", func(t *testing.T) {
		var result map[string]any
		err := processor.Parse(`{"key":"value"}`, &result)
		if err != nil {
			t.Errorf("Parse failed: %v", err)
		}
		if result["key"] != "value" {
			t.Errorf("Result[key] = %v, want value", result["key"])
		}
	})

	t.Run("ParseToSlice", func(t *testing.T) {
		var result []any
		err := processor.Parse(`[1, 2, 3]`, &result)
		if err != nil {
			t.Errorf("Parse failed: %v", err)
		}
		if len(result) != 3 {
			t.Errorf("Result length = %d, want 3", len(result))
		}
	})

	t.Run("ParseNilTarget", func(t *testing.T) {
		err := processor.Parse(`{"key":"value"}`, nil)
		if err == nil {
			t.Error("Parse with nil target should return error")
		}
	})

	t.Run("ParseInvalidJSON", func(t *testing.T) {
		var result map[string]any
		err := processor.Parse(`{invalid}`, &result)
		if err == nil {
			t.Error("Parse with invalid JSON should return error")
		}
	})

	t.Run("ParseWithPreserveNumbers", func(t *testing.T) {
		opts := Config{PreserveNumbers: true}
		var result map[string]any
		err := processor.Parse(`{"num":123}`, &result, opts)
		if err != nil {
			t.Errorf("Parse with PreserveNumbers failed: %v", err)
		}
	})
}

// TestProcessorBufferMethods tests buffer operation methods
func TestProcessorBufferMethods(t *testing.T) {
	processor, _ := New()
	defer processor.Close()

	t.Run("CompactBuffer", func(t *testing.T) {
		var dst bytes.Buffer
		src := []byte(`{"key": "value"}`)
		err := processor.CompactBuffer(&dst, src)
		if err != nil {
			t.Errorf("CompactBuffer failed: %v", err)
		}
		if dst.Len() == 0 {
			t.Error("CompactBuffer should write to dst")
		}
	})

	t.Run("Indent", func(t *testing.T) {
		var dst bytes.Buffer
		src := []byte(`{"key":"value"}`)
		err := processor.Indent(&dst, src, "", "  ")
		if err != nil {
			t.Errorf("Indent failed: %v", err)
		}
		if !strings.Contains(dst.String(), "\n") {
			t.Error("Indent should produce indented output")
		}
	})

	t.Run("HTMLEscape", func(t *testing.T) {
		var dst bytes.Buffer
		src := []byte(`{"html":"<script>"}`)
		processor.HTMLEscape(&dst, src)
		if dst.Len() == 0 {
			t.Error("HTMLEscape should write to dst")
		}
	})

	t.Run("HTMLEscapeInvalidJSON", func(t *testing.T) {
		var dst bytes.Buffer
		src := []byte(`{invalid}`)
		processor.HTMLEscape(&dst, src)
		// Should write original content on error
		if dst.String() != string(src) {
			t.Error("HTMLEscape should write original on error")
		}
	})
}

// ============================================================================
// RECURSIVE PROCESSOR TESTS
// ============================================================================

// TestDeepCopy is covered by TestDeepCopyExtended in test_helpers_new_test.go

// ============================================================================
// BATCH OPERATIONS TESTS
// ============================================================================

// ============================================================================
// NUMBER PRESERVING DECODER TESTS
// ============================================================================

// TestNumberPreservingDecoder tests numberPreservingDecoder
func TestNumberPreservingDecoder(t *testing.T) {
	decoder := newNumberPreservingDecoder(true)

	t.Run("DecodeInteger", func(t *testing.T) {
		result, err := decoder.DecodeToAny(`42`)
		if err != nil {
			t.Fatalf("DecodeToAny failed: %v", err)
		}
		// Number preservation means the literal must NOT collapse to float64.
		if _, isFloat := result.(float64); isFloat {
			t.Errorf("integer 42 decoded as float64; number preservation broken: %#v", result)
		}
	})

	t.Run("DecodeFloat", func(t *testing.T) {
		result, err := decoder.DecodeToAny(`3.14`)
		if err != nil {
			t.Fatalf("DecodeToAny failed: %v", err)
		}
		if _, isFloat := result.(float64); isFloat {
			t.Errorf("float 3.14 decoded as float64; number preservation broken: %#v", result)
		}
	})

	t.Run("DecodeObject", func(t *testing.T) {
		result, err := decoder.DecodeToAny(`{"num":42}`)
		if err != nil {
			t.Errorf("DecodeToAny failed: %v", err)
		}
		m, ok := result.(map[string]any)
		if !ok {
			t.Fatalf("Result should be map, got %T", result)
		}
		if _, ok := m["num"]; !ok {
			t.Error("num key should exist")
		}
	})

	t.Run("DecodeArray", func(t *testing.T) {
		result, err := decoder.DecodeToAny(`[1,2,3]`)
		if err != nil {
			t.Errorf("DecodeToAny failed: %v", err)
		}
		arr, ok := result.([]any)
		if !ok {
			t.Fatalf("Result should be array, got %T", result)
		}
		if len(arr) != 3 {
			t.Errorf("Array length = %d, want 3", len(arr))
		}
	})

	t.Run("DecodeInvalidJSON", func(t *testing.T) {
		_, err := decoder.DecodeToAny(`{invalid}`)
		if err == nil {
			t.Error("DecodeToAny should return error for invalid JSON")
		}
	})
}

// TestPreservingUnmarshal tests preservingUnmarshal function
func TestEncodeStringSpecialChars(t *testing.T) {
	processor, _ := New()
	defer processor.Close()

	tests := []struct {
		name  string
		input any
	}{
		{"StringWithNewline", map[string]any{"text": "line1\nline2"}},
		{"StringWithTab", map[string]any{"text": "col1\tcol2"}},
		{"StringWithQuotes", map[string]any{"text": `say "hello"`}},
		{"StringWithBackslash", map[string]any{"text": `path\to\file`}},
		{"StringWithUnicode", map[string]any{"text": "Hello 世界"}},
		{"StringWithControlChars", map[string]any{"text": string([]byte{0x01, 0x02})}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := processor.EncodeWithConfig(tt.input, DefaultConfig())
			if err != nil {
				t.Errorf("EncodeWithConfig failed: %v", err)
			}
			// Verify it can be decoded back
			var decoded map[string]any
			err = processor.Unmarshal([]byte(result), &decoded)
			if err != nil {
				t.Errorf("Failed to decode result: %v", err)
			}
		})
	}
}

// TestEncodeStructEdgeCases tests encoding struct edge cases
func TestEncodeStructEdgeCases(t *testing.T) {
	processor, _ := New()
	defer processor.Close()

	t.Run("StructWithOmitEmpty", func(t *testing.T) {
		type TestStruct struct {
			Name    string `json:"name"`
			Skipped string `json:"skipped,omitempty"`
			Value   int    `json:"value,omitempty"`
		}

		data := TestStruct{Name: "test"}
		result, err := processor.EncodeWithConfig(data, DefaultConfig())
		if err != nil {
			t.Errorf("EncodeWithConfig failed: %v", err)
		}
		if strings.Contains(result, "skipped") {
			t.Error("OmitEmpty field should be omitted when empty")
		}
	})

	t.Run("StructWithAllFieldsEmpty", func(t *testing.T) {
		type TestStruct struct {
			Name  string `json:"name,omitempty"`
			Value int    `json:"value,omitempty"`
		}

		data := TestStruct{}
		result, err := processor.EncodeWithConfig(data, DefaultConfig())
		if err != nil {
			t.Fatalf("EncodeWithConfig failed: %v", err)
		}
		// All fields are empty with omitempty, so nothing should be serialized.
		var decoded map[string]any
		if err := json.Unmarshal([]byte(result), &decoded); err != nil {
			t.Fatalf("result is not valid JSON: %v (got %q)", err, result)
		}
		if len(decoded) != 0 {
			t.Errorf("all-omitempty empty struct should encode to {}, got %q", result)
		}
	})

	t.Run("StructWithNestedStruct", func(t *testing.T) {
		type Inner struct {
			Value string `json:"value"`
		}
		type Outer struct {
			Name  string `json:"name"`
			Inner Inner  `json:"inner"`
		}

		data := Outer{Name: "outer", Inner: Inner{Value: "inner"}}
		result, err := processor.EncodeWithConfig(data, DefaultConfig())
		if err != nil {
			t.Errorf("EncodeWithConfig failed: %v", err)
		}
		if !strings.Contains(result, "inner") {
			t.Error("Result should contain nested struct")
		}
	})

	t.Run("StructWithPointerFields", func(t *testing.T) {
		type TestStruct struct {
			Name  *string `json:"name"`
			Value *int    `json:"value"`
		}

		name := "test"
		data := TestStruct{Name: &name}
		result, err := processor.EncodeWithConfig(data, DefaultConfig())
		if err != nil {
			t.Errorf("EncodeWithConfig failed: %v", err)
		}
		if !strings.Contains(result, "test") {
			t.Error("Result should contain pointer value")
		}
	})
}

// TestValidateNumberEdgeCases tests numeric schema validation (Minimum/Maximum).
// JSON numbers decode to float64, so "number" and "integer" validate identically;
// the previous Number/Integer subtests were an exact copy-paste and are now one
// table that also covers the out-of-range rejection paths.
func TestValidateNumberEdgeCases(t *testing.T) {
	processor, _ := New()
	defer processor.Close()

	schema := &Schema{
		Type:    "number",
		Minimum: 0,
		Maximum: 100,
	}
	schema.hasMinimum = true
	schema.hasMaximum = true

	tests := []struct {
		name      string
		jsonStr   string
		expectErr bool
	}{
		{"mid-range", `50`, false},
		{"at minimum", `0`, false},
		{"at maximum", `100`, false},
		{"below minimum", `-1`, true},
		{"above maximum", `101`, true},
		{"float in range", `19.99`, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs, err := processor.ValidateSchema(tt.jsonStr, schema)
			if err != nil {
				t.Fatalf("ValidateSchema failed for %s: %v", tt.jsonStr, err)
			}
			if (len(errs) > 0) != tt.expectErr {
				t.Errorf("ValidateSchema(%s) errors = %v, expectErr = %v", tt.jsonStr, errs, tt.expectErr)
			}
		})
	}
}

// TestIsEmptyFunction tests isEmpty function indirectly
// TestAssignResult tests assignResult function indirectly
func TestAssignResult(t *testing.T) {
	processor, _ := New()
	defer processor.Close()

	t.Run("AssignToInt", func(t *testing.T) {
		var result int
		err := processor.Unmarshal([]byte(`42`), &result)
		if err != nil {
			t.Errorf("Unmarshal failed: %v", err)
		}
		if result != 42 {
			t.Errorf("Result = %d, want 42", result)
		}
	})

	t.Run("AssignToFloat", func(t *testing.T) {
		var result float64
		err := processor.Unmarshal([]byte(`3.14`), &result)
		if err != nil {
			t.Errorf("Unmarshal failed: %v", err)
		}
		if result < 3.13 || result > 3.15 {
			t.Errorf("Result = %v, want approximately 3.14", result)
		}
	})

	t.Run("AssignToBool", func(t *testing.T) {
		var result bool
		err := processor.Unmarshal([]byte(`true`), &result)
		if err != nil {
			t.Errorf("Unmarshal failed: %v", err)
		}
		if !result {
			t.Error("Result should be true")
		}
	})

	t.Run("AssignToString", func(t *testing.T) {
		var result string
		err := processor.Unmarshal([]byte(`"hello"`), &result)
		if err != nil {
			t.Errorf("Unmarshal failed: %v", err)
		}
		if result != "hello" {
			t.Errorf("Result = %s, want hello", result)
		}
	})

	t.Run("AssignToSlice", func(t *testing.T) {
		var result []int
		err := processor.Unmarshal([]byte(`[1, 2, 3]`), &result)
		if err != nil {
			t.Errorf("Unmarshal failed: %v", err)
		}
		if len(result) != 3 {
			t.Errorf("Result length = %d, want 3", len(result))
		}
	})

	t.Run("AssignToMap", func(t *testing.T) {
		var result map[string]int
		err := processor.Unmarshal([]byte(`{"a": 1, "b": 2}`), &result)
		if err != nil {
			t.Errorf("Unmarshal failed: %v", err)
		}
		if result["a"] != 1 || result["b"] != 2 {
			t.Errorf("Result = %v, want {a:1, b:2}", result)
		}
	})
}

// TestMoreMethod tests More method of Decoder
func TestEncodeStringEdgeCases(t *testing.T) {
	processor, _ := New()
	defer processor.Close()

	tests := []struct {
		name  string
		input map[string]any
	}{
		{"StringWithHighUnicode", map[string]any{"text": "\U0001F600"}}, // Emoji
		{"StringWithNull", map[string]any{"text": string([]byte{0})}},
		{"StringWithBell", map[string]any{"text": string([]byte{7})}},
		{"StringWithFormFeed", map[string]any{"text": string([]byte{12})}},
		{"EmptyString", map[string]any{"text": ""}},
		{"VeryLongString", map[string]any{"text": strings.Repeat("a", 10000)}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := processor.EncodeWithConfig(tt.input, DefaultConfig())
			if err != nil {
				t.Errorf("EncodeWithConfig failed: %v", err)
			}
			// Verify it can be decoded back
			var decoded map[string]any
			err = processor.Unmarshal([]byte(result), &decoded)
			if err != nil {
				t.Errorf("Failed to decode: %v", err)
			}
		})
	}
}

// TestEncodeArrayEdgeCases tests array encoding edge cases
func TestEncodeArrayEdgeCases(t *testing.T) {
	processor, _ := New()
	defer processor.Close()

	t.Run("EmptyArray", func(t *testing.T) {
		data := map[string]any{"items": []any{}}
		result, err := processor.EncodeWithConfig(data, DefaultConfig())
		if err != nil {
			t.Errorf("EncodeWithConfig failed: %v", err)
		}
		if !strings.Contains(result, "[]") {
			t.Error("Result should contain empty array")
		}
	})

	t.Run("NestedArrays", func(t *testing.T) {
		data := map[string]any{
			"matrix": [][]any{
				{1, 2, 3},
				{4, 5, 6},
			},
		}
		result, err := processor.EncodeWithConfig(data, DefaultConfig())
		if err != nil {
			t.Errorf("EncodeWithConfig failed: %v", err)
		}
		if !strings.Contains(result, "[[") {
			t.Error("Result should contain nested arrays")
		}
	})

	t.Run("MixedArray", func(t *testing.T) {
		data := map[string]any{
			"mixed": []any{1, "two", 3.0, true, nil, map[string]any{"key": "value"}},
		}
		result, err := processor.EncodeWithConfig(data, DefaultConfig())
		if err != nil {
			t.Fatalf("EncodeWithConfig failed: %v", err)
		}
		// Every element type must survive the round trip.
		for _, want := range []string{`"two"`, "true", "null", `"key":"value"`} {
			if !strings.Contains(result, want) {
				t.Errorf("result %q missing %s", result, want)
			}
		}
	})
}

// TestEncodeMapEdgeCases tests map encoding edge cases
func TestEncodeMapEdgeCases(t *testing.T) {
	processor, _ := New()
	defer processor.Close()

	t.Run("EmptyMap", func(t *testing.T) {
		data := map[string]any{"obj": map[string]any{}}
		result, err := processor.EncodeWithConfig(data, DefaultConfig())
		if err != nil {
			t.Errorf("EncodeWithConfig failed: %v", err)
		}
		if !strings.Contains(result, "{}") {
			t.Error("Result should contain empty object")
		}
	})

	t.Run("NestedMaps", func(t *testing.T) {
		data := map[string]any{
			"level1": map[string]any{
				"level2": map[string]any{
					"level3": "deep",
				},
			},
		}
		result, err := processor.EncodeWithConfig(data, DefaultConfig())
		if err != nil {
			t.Fatalf("EncodeWithConfig failed: %v", err)
		}
		// Nested key and deepest value must both be present.
		for _, want := range []string{`"level3"`, `"deep"`} {
			if !strings.Contains(result, want) {
				t.Errorf("result %q missing %s", result, want)
			}
		}
	})

	t.Run("MapWithNumericKeys", func(t *testing.T) {
		// Map with non-string keys should still encode
		data := map[int]string{1: "one", 2: "two"}
		result, err := processor.EncodeWithConfig(data, DefaultConfig())
		if err != nil {
			t.Fatalf("EncodeWithConfig failed: %v", err)
		}
		// Numeric keys become JSON object keys; values must round-trip.
		for _, want := range []string{`"one"`, `"two"`} {
			if !strings.Contains(result, want) {
				t.Errorf("result %q missing %s", result, want)
			}
		}
	})
}

// TestValidateStringComprehensive tests string validation more comprehensively
func TestValidateStringComprehensive(t *testing.T) {
	processor, _ := New()
	defer processor.Close()

	t.Run("PatternValidation", func(t *testing.T) {
		schema := &Schema{
			Type:    "string",
			Pattern: `^[A-Z]{3}-\d{4}$`,
		}

		tests := []struct {
			jsonStr   string
			expectErr bool
		}{
			{`"ABC-1234"`, false},
			{`"XYZ-9999"`, false},
			{`"abc-1234"`, true},
			{`"ABC1234"`, true},
		}

		for _, tt := range tests {
			errors, err := processor.ValidateSchema(tt.jsonStr, schema)
			if err != nil {
				t.Errorf("ValidateSchema failed: %v", err)
				continue
			}
			hasErrors := len(errors) > 0
			if hasErrors != tt.expectErr {
				t.Errorf("ValidateSchema(%s) errors = %v, expectErr = %v", tt.jsonStr, errors, tt.expectErr)
			}
		}
	})

	t.Run("LengthValidation", func(t *testing.T) {
		schema := &Schema{
			Type:      "string",
			MinLength: 3,
			MaxLength: 10,
		}
		schema.hasMinLength = true
		schema.hasMaxLength = true

		tests := []struct {
			jsonStr   string
			expectErr bool
		}{
			{`"abc"`, false},
			{`"abcdefghij"`, false},
			{`"ab"`, true},
			{`"abcdefghijk"`, true},
		}

		for _, tt := range tests {
			errors, err := processor.ValidateSchema(tt.jsonStr, schema)
			if err != nil {
				t.Errorf("ValidateSchema failed: %v", err)
				continue
			}
			hasErrors := len(errors) > 0
			if hasErrors != tt.expectErr {
				t.Errorf("ValidateSchema(%s) errors = %v, expectErr = %v", tt.jsonStr, errors, tt.expectErr)
			}
		}
	})
}

// TestTokenMethod tests Token method of Decoder
func TestTokenMethod(t *testing.T) {
	t.Run("ReadTokens", func(t *testing.T) {
		decoder := NewDecoder(strings.NewReader(`{"key": [1, 2, 3]}`))

		var tokens []Token
		for decoder.More() {
			token, err := decoder.Token()
			if err != nil {
				break
			}
			tokens = append(tokens, token)
		}

		if len(tokens) == 0 {
			t.Error("Should have read some tokens")
		}
	})
}

// TestDecodeEdgeCases tests Decode edge cases
func TestDecodeEdgeCases(t *testing.T) {
	t.Run("DecodeIntoInterface", func(t *testing.T) {
		decoder := NewDecoder(strings.NewReader(`{"key": "value"}`))

		var result any
		err := decoder.Decode(&result)
		if err != nil {
			t.Errorf("Decode failed: %v", err)
		}

		m, ok := result.(map[string]any)
		if !ok {
			t.Fatal("Result should be a map")
		}
		if m["key"] != "value" {
			t.Errorf("key = %v, want value", m["key"])
		}
	})

	t.Run("MultipleDecodes", func(t *testing.T) {
		decoder := NewDecoder(strings.NewReader(`1 "two" 3.0`))

		var results []any
		for decoder.More() {
			var v any
			err := decoder.Decode(&v)
			if err != nil {
				break
			}
			results = append(results, v)
		}

		if len(results) < 2 {
			t.Error("Should have decoded multiple values")
		}
	})
}

// TestProcessorEncodeWithConfig tests Processor.EncodeWithConfig function
func TestProcessorEncodeWithConfig(t *testing.T) {
	processor, _ := New()
	defer processor.Close()

	config := DefaultConfig()
	config.Pretty = true
	config.Indent = "    "

	data := map[string]any{"key": "value"}
	result, err := processor.EncodeWithConfig(data, config)
	if err != nil {
		t.Errorf("EncodeWithConfig failed: %v", err)
	}

	if !strings.Contains(result, "    ") {
		t.Error("Result should use custom indent")
	}
}

// TestEncodeStreamWithConfig tests EncodeStream function
func TestEncodeStreamWithConfig(t *testing.T) {
	processor, _ := New()
	defer processor.Close()

	config := DefaultConfig()
	data := []map[string]any{
		{"id": 1},
		{"id": 2},
	}

	result, err := processor.EncodeStream(data, config)
	if err != nil {
		t.Errorf("EncodeStream failed: %v", err)
	}

	if !strings.Contains(result, `"id"`) {
		t.Error("Result should contain id")
	}
}

// ============================================================================
// PARSER EDGE CASES TESTS
// ============================================================================

// TestValuesEqualComprehensive tests valuesEqual function more comprehensively
func TestValuesEqualComprehensive(t *testing.T) {
	processor, _ := New()
	defer processor.Close()

	t.Run("StringEquality", func(t *testing.T) {
		schema := &Schema{
			Type:  "string",
			Const: "expected",
		}

		tests := []struct {
			jsonStr   string
			expectErr bool
		}{
			{`"expected"`, false},
			{`"other"`, true},
		}

		for _, tt := range tests {
			errors, err := processor.ValidateSchema(tt.jsonStr, schema)
			if err != nil {
				t.Errorf("ValidateSchema failed: %v", err)
				continue
			}
			hasErrors := len(errors) > 0
			if hasErrors != tt.expectErr {
				t.Errorf("ValidateSchema(%s) errors = %v, expectErr = %v", tt.jsonStr, errors, tt.expectErr)
			}
		}
	})

	t.Run("NumberEquality", func(t *testing.T) {
		schema := &Schema{
			Type:  "number",
			Const: 42.0,
		}

		tests := []struct {
			jsonStr   string
			expectErr bool
		}{
			{`42`, false},
			{`43`, true},
		}

		for _, tt := range tests {
			errors, err := processor.ValidateSchema(tt.jsonStr, schema)
			if err != nil {
				t.Errorf("ValidateSchema failed: %v", err)
				continue
			}
			hasErrors := len(errors) > 0
			if hasErrors != tt.expectErr {
				t.Errorf("ValidateSchema(%s) errors = %v, expectErr = %v", tt.jsonStr, errors, tt.expectErr)
			}
		}
	})

	t.Run("BooleanEquality", func(t *testing.T) {
		schema := &Schema{
			Type:  "boolean",
			Const: true,
		}

		tests := []struct {
			jsonStr   string
			expectErr bool
		}{
			{`true`, false},
			{`false`, true},
		}

		for _, tt := range tests {
			errors, err := processor.ValidateSchema(tt.jsonStr, schema)
			if err != nil {
				t.Errorf("ValidateSchema failed: %v", err)
				continue
			}
			hasErrors := len(errors) > 0
			if hasErrors != tt.expectErr {
				t.Errorf("ValidateSchema(%s) errors = %v, expectErr = %v", tt.jsonStr, errors, tt.expectErr)
			}
		}
	})
}

// TestValidateNumberComprehensive tests validateNumber function more comprehensively
func TestValidateNumberComprehensive(t *testing.T) {
	processor, _ := New()
	defer processor.Close()

	t.Run("MultipleOf", func(t *testing.T) {
		schema := &Schema{
			Type:       "number",
			MultipleOf: 5,
		}

		tests := []struct {
			jsonStr   string
			expectErr bool
		}{
			{`10`, false},
			{`15`, false},
			{`7`, true},
		}

		for _, tt := range tests {
			errors, err := processor.ValidateSchema(tt.jsonStr, schema)
			if err != nil {
				t.Errorf("ValidateSchema failed: %v", err)
				continue
			}
			hasErrors := len(errors) > 0
			if hasErrors != tt.expectErr {
				t.Errorf("ValidateSchema(%s) errors = %v, expectErr = %v", tt.jsonStr, errors, tt.expectErr)
			}
		}
	})
}

// TestEncodeNumberEdgeCases tests number encoding edge cases
func TestEncodeNumberEdgeCases(t *testing.T) {
	processor, _ := New()
	defer processor.Close()

	tests := []struct {
		name  string
		input any
		want  string // must appear verbatim in the encoded output
	}{
		{"LargeInt", map[string]any{"value": 9223372036854775807}, "9223372036854775807"},
		{"SmallInt", map[string]any{"value": -9223372036854775808}, "-9223372036854775808"},
		{"LargeFloat", map[string]any{"value": 1.7976931348623157e+308}, "1.7976931348623157e+308"},
		{"SmallFloat", map[string]any{"value": -1.7976931348623157e+308}, "-1.7976931348623157e+308"},
		{"NegativeZero", map[string]any{"value": 0.0}, "0"},
		{"VerySmallFloat", map[string]any{"value": 1e-300}, "1e-300"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := processor.EncodeWithConfig(tt.input, DefaultConfig())
			if err != nil {
				t.Fatalf("EncodeWithConfig failed: %v", err)
			}
			if !strings.Contains(result, tt.want) {
				t.Errorf("result %q missing %q", result, tt.want)
			}
		})
	}
}

// TestCustomEncoderEdgeCases tests customEncoder edge cases
func TestCustomEncoderEdgeCases(t *testing.T) {
	t.Run("EncodeWithCustomConfig", func(t *testing.T) {
		config := Config{
			Pretty:          true,
			Indent:          "  ",
			EscapeHTML:      false,
			SortKeys:        true,
			ValidateUTF8:    true,
			MaxDepth:        50,
			PreserveNumbers: true,
			EscapeUnicode:   false,
			IncludeNulls:    true,
		}

		encoder := newCustomEncoder(config)
		data := map[string]any{
			"html": "<script>",
			"num":  42,
		}

		result, err := encoder.Encode(data)
		if err != nil {
			t.Errorf("Encode failed: %v", err)
		}

		if !strings.Contains(result, "html") {
			t.Error("Result should contain html key")
		}

		encoder.Close()
	})

	t.Run("EncodeNil", func(t *testing.T) {
		config := DefaultConfig()
		encoder := newCustomEncoder(config)

		result, err := encoder.Encode(nil)
		if err != nil {
			t.Errorf("Encode failed: %v", err)
		}

		if result != "null" {
			t.Errorf("Result = %s, want null", result)
		}

		encoder.Close()
	})
}

// TestStringFormatValidation tests string format validation
func TestTypedGetters_DefaultContract(t *testing.T) {
	t.Run("string", func(t *testing.T) {
		cases := []struct{ name, in, path, def, want string }{
			{"Found", `{"name":"Alice"}`, "name", "default", "Alice"},
			{"NotFound", `{"name":"Alice"}`, "missing", "default", "default"},
			{"InvalidJSON", `{invalid}`, "name", "default", "default"},
		}
		for _, c := range cases {
			if got := GetString(c.in, c.path, c.def); got != c.want {
				t.Errorf("%s: GetString = %q, want %q", c.name, got, c.want)
			}
		}
	})

	t.Run("int", func(t *testing.T) {
		cases := []struct {
			name, in, path string
			def, want      int
		}{
			{"Found", `{"count":42}`, "count", -1, 42},
			{"NotFound", `{"count":42}`, "missing", -1, -1},
		}
		for _, c := range cases {
			if got := GetInt(c.in, c.path, c.def); got != c.want {
				t.Errorf("%s: GetInt = %d, want %d", c.name, got, c.want)
			}
		}
	})

	t.Run("float", func(t *testing.T) {
		cases := []struct {
			name, in, path string
			def, want      float64
		}{
			{"Found", `{"value":3.14}`, "value", -1.0, 3.14},
			{"NotFound", `{"value":3.14}`, "missing", -1.0, -1.0},
			{"IntToFloat", `{"count":42}`, "count", -1.0, 42.0},
		}
		for _, c := range cases {
			if got := GetFloat(c.in, c.path, c.def); got != c.want {
				t.Errorf("%s: GetFloat = %v, want %v", c.name, got, c.want)
			}
		}
	})

	t.Run("bool", func(t *testing.T) {
		cases := []struct {
			name, in, path string
			def, want      bool
		}{
			{"FoundTrue", `{"active":true}`, "active", false, true},
			{"FoundFalse", `{"active":false}`, "active", true, false},
			{"NotFound", `{"active":true}`, "missing", false, false},
		}
		for _, c := range cases {
			if got := GetBool(c.in, c.path, c.def); got != c.want {
				t.Errorf("%s: GetBool = %v, want %v", c.name, got, c.want)
			}
		}
	})
}

// TestValidWithConfig tests ValidWithConfig function
func TestValidWithConfig(t *testing.T) {
	t.Run("WithConfig", func(t *testing.T) {
		cfg := Config{
			MaxNestingDepthSecurity: 100,
			MaxJSONSize:             1024 * 1024,
		}

		tests := []struct {
			jsonStr  string
			expected bool
		}{
			{`{"key":"value"}`, true},
			{`{invalid}`, false},
		}

		for _, tt := range tests {
			result, _ := ValidWithConfig(tt.jsonStr, cfg)
			if result != tt.expected {
				t.Errorf("ValidWithConfig(%q) = %v, want %v", tt.jsonStr, result, tt.expected)
			}
		}
	})
}

// TestParseTopLevel tests the top-level Parse function
func TestParseTopLevel(t *testing.T) {
	t.Run("ParseToAny", func(t *testing.T) {
		result, err := ParseAny(`{"key":"value"}`)
		if err != nil {
			t.Errorf("Parse failed: %v", err)
		}
		m, ok := result.(map[string]any)
		if !ok {
			t.Fatalf("Expected map, got %T", result)
		}
		if m["key"] != "value" {
			t.Errorf("Result[key] = %v, want value", m["key"])
		}
	})

	t.Run("ParseToArray", func(t *testing.T) {
		result, err := ParseAny(`[1, 2, 3]`)
		if err != nil {
			t.Errorf("Parse failed: %v", err)
		}
		arr, ok := result.([]any)
		if !ok {
			t.Fatalf("Expected array, got %T", result)
		}
		if len(arr) != 3 {
			t.Errorf("Result length = %d, want 3", len(arr))
		}
	})

	t.Run("ParseInvalidJSON", func(t *testing.T) {
		_, err := ParseAny(`{invalid}`)
		if err == nil {
			t.Error("Parse with invalid JSON should return error")
		}
	})

	t.Run("ParseIntoStruct", func(t *testing.T) {
		type testUser struct {
			Name string `json:"name"`
			Age  int    `json:"age"`
		}
		var user testUser
		err := Parse(`{"name":"Alice","age":30}`, &user)
		if err != nil {
			t.Fatalf("Parse into struct failed: %v", err)
		}
		if user.Name != "Alice" {
			t.Errorf("user.Name = %q, want Alice", user.Name)
		}
		if user.Age != 30 {
			t.Errorf("user.Age = %d, want 30", user.Age)
		}
	})

	t.Run("ParseIntoMap", func(t *testing.T) {
		var obj map[string]any
		err := Parse(`{"key":"value"}`, &obj)
		if err != nil {
			t.Fatalf("Parse into map failed: %v", err)
		}
		if obj["key"] != "value" {
			t.Errorf("obj[key] = %v, want value", obj["key"])
		}
	})
}

// ============================================================================
// TOP-LEVEL FILE FUNCTIONS - Missing coverage tests
// ============================================================================

// TestMarshalToFileTopLevel tests the top-level MarshalToFile function
func TestMarshalToFileTopLevel(t *testing.T) {
	t.Run("MarshalAndLoad", func(t *testing.T) {
		tempDir := t.TempDir()
		filePath := filepath.Join(tempDir, "marshal_test.json")
		testData := map[string]any{"key": "value"}

		err := MarshalToFile(filePath, testData)
		if err != nil {
			t.Errorf("MarshalToFile failed: %v", err)
		}

		loaded, err := LoadFromFile(filePath)
		if err != nil {
			t.Errorf("LoadFromFile failed: %v", err)
		}

		if !strings.Contains(loaded, `"key"`) {
			t.Error("Loaded data should contain 'key'")
		}
	})
}

// TestSaveToWriterTopLevel tests the top-level SaveToWriter function
func TestSaveToWriterTopLevel(t *testing.T) {
	t.Run("SaveToBuffer", func(t *testing.T) {
		var buf bytes.Buffer
		testData := map[string]any{"key": "value"}
		cfg := DefaultConfig()

		err := SaveToWriter(&buf, testData, cfg)
		if err != nil {
			t.Errorf("SaveToWriter failed: %v", err)
		}

		if !strings.Contains(buf.String(), `"key"`) {
			t.Error("Buffer should contain 'key'")
		}
	})
}

// ============================================================================
// TEST HELPERS - Missing coverage tests
// ============================================================================

// ============================================================================
// RESOURCE MANAGER - Removed (drainPool was dead code, tests removed)
// ============================================================================
// EDGE CASES - Additional boundary tests
// ============================================================================

// TestArrayBoundaryConditions tests array boundary conditions
func TestArrayBoundaryConditions(t *testing.T) {
	processor, _ := New()
	defer processor.Close()

	jsonStr := `{"items": [1, 2, 3]}`

	t.Run("NegativeIndex", func(t *testing.T) {
		result, err := processor.Get(jsonStr, "items[-1]")
		if err != nil {
			t.Errorf("Get with negative index failed: %v", err)
		}
		if result != 3.0 {
			t.Errorf("items[-1] = %v, want 3", result)
		}
	})

	t.Run("IndexOutOfRange", func(t *testing.T) {
		result, err := processor.Get(jsonStr, "items[100]")
		// Out-of-range may return nil or error depending on implementation
		if err == nil && result != nil {
			t.Errorf("out-of-range index should return nil or error, got result=%v", result)
		}
	})

	t.Run("EmptyArray", func(t *testing.T) {
		result, err := processor.Get(`{"empty": []}`, "empty")
		if err != nil {
			t.Errorf("Get empty array failed: %v", err)
		}
		arr, ok := result.([]any)
		if !ok || len(arr) != 0 {
			t.Error("Empty array should be empty")
		}
	})
}

// ============================================================================
// FAST ENCODER TESTS - Missing coverage
// ============================================================================

// TestFastEncoderFunctions tests the fast encoder functions
func TestFastEncoderFunctions(t *testing.T) {
	t.Run("FastEncodeSimpleToBytes", func(t *testing.T) {
		data := map[string]any{"key": "value", "num": 123}
		result, ok := getDefaultProcessor().fastEncodeSimpleToBytes(data, DefaultMaxDepth)
		if !ok {
			t.Error("fastEncodeSimpleToBytes should succeed for simple data")
		}
		if !strings.Contains(string(result), `"key"`) {
			t.Error("Result should contain key")
		}
	})

	t.Run("FastEncodeSimpleToBytesWithHTMLEscape", func(t *testing.T) {
		data := map[string]any{"html": "<script>"}
		result, ok := getDefaultProcessor().fastEncodeSimpleToBytes(data, DefaultMaxDepth)
		if !ok {
			t.Error("fastEncodeSimpleToBytes should succeed")
		}
		if !strings.Contains(string(result), "\\u003c") {
			t.Error("HTML should be escaped")
		}
	})
}

// TestConfigValidationEdgeCases tests Config.Validate edge cases
func TestConfigValidationEdgeCases(t *testing.T) {
	t.Run("ValidateWithWarnings", func(t *testing.T) {
		cfg := Config{
			MaxCacheSize: -1,
			MaxJSONSize:  -1,
		}
		warnings := cfg.ValidateWithWarnings()
		// Should have warnings for negative values
		if len(warnings) == 0 {
			t.Errorf("expected warnings for negative MaxCacheSize/MaxJSONSize, got none")
		}
	})

	t.Run("ValidateZeroConfig", func(t *testing.T) {
		cfg := Config{}
		err := cfg.Validate()
		if err != nil {
			t.Errorf("Validate of zero config should succeed: %v", err)
		}
	})
}

// TestStreamEncoderDecodeRoundTrip tests encoding and decoding round trip
func TestStreamEncoderDecodeRoundTrip(t *testing.T) {
	t.Run("RoundTrip", func(t *testing.T) {
		var buf bytes.Buffer

		// Encode
		encoder := NewEncoder(&buf)
		original := map[string]any{
			"name":  "test",
			"value": 123,
			"nested": map[string]any{
				"key": "nested_value",
			},
		}
		err := encoder.Encode(original)
		if err != nil {
			t.Fatalf("Encode failed: %v", err)
		}

		// Decode
		decoder := NewDecoder(&buf)
		var decoded map[string]any
		err = decoder.Decode(&decoded)
		if err != nil {
			t.Fatalf("Decode failed: %v", err)
		}

		if decoded["name"] != "test" {
			t.Error("Decoded name should be 'test'")
		}
	})
}

// TestTypeConversionErrors tests type conversion error conditions
func TestTypeConversionErrors(t *testing.T) {
	processor, _ := New()
	defer processor.Close()

	t.Run("InvalidJSONGet", func(t *testing.T) {
		_, err := processor.Get(`{invalid}`, "key")
		if err == nil {
			t.Error("Get with invalid JSON should return error")
		}
	})

	t.Run("InvalidJSONSet", func(t *testing.T) {
		_, err := processor.Set(`{invalid}`, "key", "value")
		if err == nil {
			t.Error("Set with invalid JSON should return error")
		}
	})

	t.Run("InvalidJSONDelete", func(t *testing.T) {
		_, err := processor.Delete(`{invalid}`, "key")
		if err == nil {
			t.Error("Delete with invalid JSON should return error")
		}
	})

	t.Run("TypeConversionFailures", func(t *testing.T) {
		// Test type conversion error handling
		_, ok := convertToInt("not a number")
		if ok {
			t.Error("Converting non-numeric string to int should fail")
		}

		_, ok = convertToFloat64("not a number")
		if ok {
			t.Error("Converting non-numeric string to float should fail")
		}

		_, ok = convertToBool("maybe")
		if ok {
			t.Error("Converting invalid bool string should fail")
		}
	})
}

// ============================================================================
// API wrapper tests for 0% coverage functions
// ============================================================================

// TestAPISetCreate tests the package-level SetCreate function
func TestAPIDeleteClean(t *testing.T) {
	t.Run("delete and clean array", func(t *testing.T) {
		result, err := DeleteClean(`{"items":[1,2,3]}`, "items[1]")
		if err != nil {
			t.Fatalf("DeleteClean error: %v", err)
		}
		if result == "" {
			t.Error("result should not be empty")
		}
	})

	t.Run("delete and clean object", func(t *testing.T) {
		result, err := DeleteClean(`{"a":1,"b":null}`, "b")
		if err != nil {
			t.Fatalf("DeleteClean error: %v", err)
		}
		assertJSONEqual(t, `{"a":1}`, result)
	})
}

// TestAPILoadFromReader tests the package-level LoadFromReader function
func TestAPILoadFromReader(t *testing.T) {
	t.Run("valid JSON", func(t *testing.T) {
		result, err := LoadFromReader(strings.NewReader(`{"key":"value"}`))
		if err != nil {
			t.Fatalf("LoadFromReader error: %v", err)
		}
		if result == "" {
			t.Error("result should not be empty")
		}
	})

	t.Run("empty reader", func(t *testing.T) {
		result, err := LoadFromReader(strings.NewReader(""))
		if err != nil {
			t.Errorf("unexpected error for empty reader: %v", err)
		}
		if result != "" {
			t.Errorf("expected empty result, got %q", result)
		}
	})
}

// TestAPIClearCache tests the package-level ClearCache function
func TestAPIClearCache(t *testing.T) {
	// Should not panic
	ClearCache()

	// Verify operations still work after cache clear
	_, err := Get(`{"a":1}`, "a")
	if err != nil {
		t.Errorf("Get after ClearCache error: %v", err)
	}
}

// ============================================================================
// LOW-COVERAGE ENCODING TESTS - Additional coverage
// Target: Number.Int64, Number.Float64, Decoder.More/Buffered/InputOffset,
//         parseString, parseNumber, escapeRune, truncateFloat, valuesEqual
// ============================================================================

// TestNumberMethods tests Number.Int64() and Number.Float64() methods
func TestNumberMethods(t *testing.T) {
	t.Run("Int64", func(t *testing.T) {
		tests := []struct {
			name    string
			num     Number
			want    int64
			wantErr bool
		}{
			{"ValidInteger", Number("42"), 42, false},
			{"NegativeInteger", Number("-7"), -7, false},
			{"Zero", Number("0"), 0, false},
			{"LargeInteger", Number("9223372036854775807"), 9223372036854775807, false},
			{"InvalidString", Number("invalid"), 0, true},
			{"FloatString", Number("42.5"), 0, true},
			{"Empty", Number(""), 0, true},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				got, err := tt.num.Int64()
				if tt.wantErr {
					if err == nil {
						t.Error("expected error, got nil")
					}
					return
				}
				if err != nil {
					t.Errorf("unexpected error: %v", err)
					return
				}
				if got != tt.want {
					t.Errorf("Int64() = %d, want %d", got, tt.want)
				}
			})
		}
	})

	t.Run("Float64", func(t *testing.T) {
		tests := []struct {
			name    string
			num     Number
			want    float64
			wantErr bool
		}{
			{"ValidFloat", Number("42.5"), 42.5, false},
			{"IntegerAsFloat", Number("42"), 42.0, false},
			{"NegativeFloat", Number("-3.14"), -3.14, false},
			{"Scientific", Number("1e10"), 1e10, false},
			{"Zero", Number("0"), 0.0, false},
			{"InvalidString", Number("invalid"), 0, true},
			{"Empty", Number(""), 0, true},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				got, err := tt.num.Float64()
				if tt.wantErr {
					if err == nil {
						t.Error("expected error, got nil")
					}
					return
				}
				if err != nil {
					t.Errorf("unexpected error: %v", err)
					return
				}
				if got != tt.want {
					t.Errorf("Float64() = %v, want %v", got, tt.want)
				}
			})
		}
	})

	t.Run("String", func(t *testing.T) {
		// String() returns the underlying literal unchanged.
		for _, lit := range []string{"123.45", "42", "-3.14", "1e10", "0", ""} {
			if got := Number(lit).String(); got != lit {
				t.Errorf("Number(%q).String() = %q, want %q", lit, got, lit)
			}
		}
	})
}

// TestDecoderMoreBufferedOffset tests Decoder.More(), Buffered(), InputOffset()
func TestDecoderMoreBufferedOffset(t *testing.T) {
	t.Run("MoreInArray", func(t *testing.T) {
		dec := NewDecoder(strings.NewReader(`[1,2,3]`))

		tok, err := dec.Token()
		if err != nil {
			t.Fatalf("Token() error: %v", err)
		}
		if tok != Delim('[') {
			t.Fatalf("expected '[', got %v", tok)
		}

		// More should be true before we read all elements
		if !dec.More() {
			t.Error("More() should return true at start of array")
		}

		// Read all three numbers using Token() to handle comma-separated values
		for i := 0; i < 3; i++ {
			_, err := dec.Token()
			if err != nil {
				t.Errorf("Token(%d) error: %v", i, err)
			}
		}

		// More should be false after all elements consumed
		if dec.More() {
			t.Error("More() should return false after all elements consumed")
		}

		// Read closing bracket
		tok, err = dec.Token()
		if err != nil {
			t.Fatalf("closing Token() error: %v", err)
		}
		if tok != Delim(']') {
			t.Fatalf("expected ']', got %v", tok)
		}
	})

	t.Run("Buffered", func(t *testing.T) {
		dec := NewDecoder(strings.NewReader(`{"key":"value"}`))
		reader := dec.Buffered()
		if reader == nil {
			t.Error("Buffered() should return non-nil reader")
		}
	})

	t.Run("InputOffset", func(t *testing.T) {
		dec := NewDecoder(strings.NewReader(`  [1,2,3]`))

		// Before reading, offset should be 0
		if dec.InputOffset() != 0 {
			t.Errorf("initial InputOffset() = %d, want 0", dec.InputOffset())
		}

		// Decode should advance offset
		var result any
		if err := dec.Decode(&result); err != nil {
			t.Fatalf("Decode error: %v", err)
		}

		offset := dec.InputOffset()
		if offset == 0 {
			t.Error("InputOffset() should advance after Decode")
		}
	})

	t.Run("MoreWithMultipleObjects", func(t *testing.T) {
		dec := NewDecoder(strings.NewReader(`{"a":1}{"b":2}`))

		var v1 map[string]any
		if err := dec.Decode(&v1); err != nil {
			t.Fatalf("first Decode error: %v", err)
		}

		if !dec.More() {
			t.Error("More() should return true with second object pending")
		}

		var v2 map[string]any
		if err := dec.Decode(&v2); err != nil {
			t.Fatalf("second Decode error: %v", err)
		}

		if dec.More() {
			t.Error("More() should return false after all objects consumed")
		}
	})
}

// TestDecoderParseString tests parseString through the Decoder.Token API
func TestDecoderParseString(t *testing.T) {
	t.Run("ValidEscapes", func(t *testing.T) {
		tests := []struct {
			name  string
			input string
			want  string
		}{
			{"Newline", `"hello\nworld"`, "hello\nworld"},
			{"Tab", `"col1\tcol2"`, "col1\tcol2"},
			{"Quote", `"say \"hello\""`, `say "hello"`},
			{"Backslash", `"path\\to\\file"`, "path\\to\\file"},
			{"UnicodeEscape", `"\u0041"`, "A"},
			{"Slash", `"a\/b"`, "a/b"},
			{"Backspace", `"a\bb"`, "a\bb"},
			{"FormFeed", `"a\fb"`, "a\fb"},
			{"CarriageReturn", `"a\rb"`, "a\rb"},
			{"SimpleString", `"hello"`, "hello"},
			{"EmptyString", `""`, ""},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				dec := NewDecoder(strings.NewReader(tt.input))
				tok, err := dec.Token()
				if err != nil {
					t.Fatalf("Token() error: %v", err)
				}
				str, ok := tok.(string)
				if !ok {
					t.Fatalf("expected string token, got %T", tok)
				}
				if str != tt.want {
					t.Errorf("got %q, want %q", str, tt.want)
				}
			})
		}
	})

	t.Run("InvalidEscape", func(t *testing.T) {
		dec := NewDecoder(strings.NewReader(`"hello\qworld"`))
		_, err := dec.Token()
		if err == nil {
			t.Error("expected error for invalid escape sequence \\q")
		}
	})

	t.Run("MultiByteUTF8", func(t *testing.T) {
		tests := []struct {
			name  string
			input string
			want  string
		}{
			{"Chinese", `"世界"`, "世界"},
			{"Emoji", `"\u0048\u0065\u006c\u006c\u006f"`, "Hello"},
			{"Mixed", `"Hello 世界"`, "Hello 世界"},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				dec := NewDecoder(strings.NewReader(tt.input))
				tok, err := dec.Token()
				if err != nil {
					t.Fatalf("Token() error: %v", err)
				}
				str, ok := tok.(string)
				if !ok {
					t.Fatalf("expected string token, got %T", tok)
				}
				if str != tt.want {
					t.Errorf("got %q, want %q", str, tt.want)
				}
			})
		}
	})
}

// TestDecoderParseNumber tests parseNumber through Decoder with UseNumber
func TestDecoderParseNumber(t *testing.T) {
	t.Run("UseNumberEnabled", func(t *testing.T) {
		tests := []struct {
			name    string
			input   string
			wantNum string
			wantErr bool
		}{
			{"Integer", `42`, "42", false},
			{"NegativeInteger", `-7`, "-7", false},
			{"Float", `3.14`, "3.14", false},
			{"ScientificLower", `1e10`, "1e10", false},
			{"ScientificUpper", `1.5E-3`, "1.5E-3", false},
			{"Zero", `0`, "0", false},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				dec := NewDecoder(strings.NewReader(tt.input))
				dec.UseNumber()
				var result any
				err := dec.Decode(&result)
				if tt.wantErr {
					if err == nil {
						t.Error("expected error, got nil")
					}
					return
				}
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				num, ok := result.(Number)
				if !ok {
					t.Fatalf("expected Number type, got %T", result)
				}
				if string(num) != tt.wantNum {
					t.Errorf("got Number(%q), want Number(%q)", num, tt.wantNum)
				}
			})
		}
	})

	t.Run("UseNumberDisabled", func(t *testing.T) {
		tests := []struct {
			name  string
			input string
			want  any
		}{
			{"IntegerAsInt64", `42`, float64(42)},
			{"Float", `3.14`, float64(3.14)},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				dec := NewDecoder(strings.NewReader(tt.input))
				var result any
				if err := dec.Decode(&result); err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				f, ok := result.(float64)
				if !ok {
					t.Fatalf("expected float64, got %T", result)
				}
				if f != tt.want {
					t.Errorf("got %v, want %v", f, tt.want)
				}
			})
		}
	})

	t.Run("NumberAtEndOfStream", func(t *testing.T) {
		dec := NewDecoder(strings.NewReader(`42`))
		dec.UseNumber()
		var result any
		if err := dec.Decode(&result); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		num, ok := result.(Number)
		if !ok {
			t.Fatalf("expected Number type, got %T", result)
		}
		if string(num) != "42" {
			t.Errorf("got Number(%q), want Number(%q)", num, "42")
		}
	})

	t.Run("ScientificNotationInArray", func(t *testing.T) {
		dec := NewDecoder(strings.NewReader(`[1e10, 1.5E-3, -2.5e+2]`))
		dec.UseNumber()
		var result any
		if err := dec.Decode(&result); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		arr, ok := result.([]any)
		if !ok {
			t.Fatalf("expected []any, got %T", result)
		}
		if len(arr) != 3 {
			t.Fatalf("expected 3 elements, got %d", len(arr))
		}
		for i, elem := range arr {
			if _, ok := elem.(Number); !ok {
				t.Errorf("element %d: expected Number, got %T", i, elem)
			}
		}
	})
}

// TestCustomEncoderEscapeRune tests escapeRune paths via EncodeWithConfig
func TestCustomEncoderEscapeRune(t *testing.T) {
	t.Run("EscapeHTML", func(t *testing.T) {
		tests := []struct {
			name        string
			input       map[string]any
			cfg         Config
			wantContain string
			dontContain string
		}{
			{
				name:        "EscapeLessThan",
				input:       map[string]any{"html": "<tag>"},
				cfg:         func() Config { c := DefaultConfig(); c.EscapeHTML = true; return c }(),
				wantContain: `\u003c`,
			},
			{
				name:        "EscapeGreaterThan",
				input:       map[string]any{"html": "a>b"},
				cfg:         func() Config { c := DefaultConfig(); c.EscapeHTML = true; return c }(),
				wantContain: `\u003e`,
			},
			{
				name:        "EscapeAmpersand",
				input:       map[string]any{"html": "a&b"},
				cfg:         func() Config { c := DefaultConfig(); c.EscapeHTML = true; return c }(),
				wantContain: `\u0026`,
			},
			{
				name:        "NoEscapeHTML",
				input:       map[string]any{"html": "<tag>"},
				cfg:         func() Config { c := DefaultConfig(); c.EscapeHTML = false; return c }(),
				dontContain: `\u003c`,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				result, err := EncodeWithConfig(tt.input, tt.cfg)
				if err != nil {
					t.Fatalf("EncodeWithConfig error: %v", err)
				}
				if tt.wantContain != "" && !strings.Contains(result, tt.wantContain) {
					t.Errorf("result %q should contain %q", result, tt.wantContain)
				}
				if tt.dontContain != "" && strings.Contains(result, tt.dontContain) {
					t.Errorf("result %q should not contain %q", result, tt.dontContain)
				}
			})
		}
	})

	t.Run("EscapeUnicode", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.EscapeUnicode = true

		result, err := EncodeWithConfig(map[string]any{"text": "cafe\u0301"}, cfg)
		if err != nil {
			t.Fatalf("EncodeWithConfig error: %v", err)
		}
		// Characters > 0x7F should be escaped to \uXXXX when EscapeUnicode is true
		if !strings.Contains(result, `\u`) {
			t.Errorf("EscapeUnicode should produce \\u escapes, got %q", result)
		}
	})

	t.Run("EscapeSlash", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.EscapeSlash = true

		result, err := EncodeWithConfig(map[string]any{"path": "a/b/c"}, cfg)
		if err != nil {
			t.Fatalf("EncodeWithConfig error: %v", err)
		}
		if !strings.Contains(result, `\/`) {
			t.Errorf("EscapeSlash should escape / to \\/, got %q", result)
		}

		// Verify without EscapeSlash
		cfgNoEscape := DefaultConfig()
		cfgNoEscape.EscapeSlash = false
		result2, err := EncodeWithConfig(map[string]any{"path": "a/b/c"}, cfgNoEscape)
		if err != nil {
			t.Fatalf("EncodeWithConfig error: %v", err)
		}
		if strings.Contains(result2, `\/`) {
			t.Errorf("without EscapeSlash, / should not be escaped, got %q", result2)
		}
	})

	t.Run("EscapeNewlinesFalse", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.EscapeNewlines = false

		result, err := EncodeWithConfig(map[string]any{"text": "line1\nline2"}, cfg)
		if err != nil {
			t.Fatalf("EncodeWithConfig error: %v", err)
		}
		// With EscapeNewlines=false, literal newline should be kept
		if !strings.Contains(result, "\n") {
			t.Errorf("EscapeNewlines=false should keep literal newline, got %q", result)
		}
		if strings.Contains(result, `\n`) {
			t.Errorf("EscapeNewlines=false should keep the newline literal, found escaped \\n in %q", result)
		}
	})

	t.Run("EscapeTabsFalse", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.EscapeTabs = false

		result, err := EncodeWithConfig(map[string]any{"text": "col1\tcol2"}, cfg)
		if err != nil {
			t.Fatalf("EncodeWithConfig error: %v", err)
		}
		// With EscapeTabs=false, literal tab should be kept
		if !strings.Contains(result, "\t") {
			t.Errorf("EscapeTabs=false should keep literal tab, got %q", result)
		}
	})

	t.Run("ControlCharacters", func(t *testing.T) {
		cfg := DefaultConfig()
		// Default config has EscapeNewlines=true, EscapeTabs=true

		result, err := EncodeWithConfig(map[string]any{"text": string([]byte{0x01, 0x02})}, cfg)
		if err != nil {
			t.Fatalf("EncodeWithConfig error: %v", err)
		}
		// Control chars < 0x20 should be escaped as \uXXXX
		if !strings.Contains(result, `\u0001`) {
			t.Errorf("control chars should be unicode-escaped, got %q", result)
		}
	})

	t.Run("ValidateUTF8", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.ValidateUTF8 = true

		// Create a string with invalid UTF-8 sequence.
		// Note: Go's for-range over string replaces invalid bytes with U+FFFD,
		// which is a valid rune, so ValidateUTF8 won't error in this path.
		// Verify the encoding succeeds and the replacement character appears.
		invalidUTF8 := string([]byte{0xff, 0xfe})
		result, err := EncodeWithConfig(map[string]any{"text": invalidUTF8}, cfg)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// The invalid bytes should be encoded (as replacement characters or escaped)
		if !strings.Contains(result, "text") {
			t.Errorf("result should contain key 'text', got %q", result)
		}
	})

	t.Run("DisableEscaping", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.DisableEscaping = true

		result, err := EncodeWithConfig(map[string]any{"text": "Hello <World> & /Friends\\"}, cfg)
		if err != nil {
			t.Fatalf("EncodeWithConfig error: %v", err)
		}
		// With DisableEscaping, HTML chars and slash should appear literally
		if !strings.Contains(result, "<") {
			t.Errorf("DisableEscaping should keep < literal, got %q", result)
		}
		if strings.Contains(result, `\u003c`) {
			t.Errorf("DisableEscaping should not unicode-escape <, got %q", result)
		}
	})
}

// TestTruncateFloatExtended tests truncateFloat via EncodeWithConfig with FloatPrecision
func TestTruncateFloatExtended(t *testing.T) {
	t.Run("FloatPrecision2", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.FloatPrecision = 2
		cfg.FloatTruncate = true

		result, err := EncodeWithConfig(map[string]any{"pi": 3.14159}, cfg)
		if err != nil {
			t.Fatalf("EncodeWithConfig error: %v", err)
		}
		if !strings.Contains(result, "3.14") {
			t.Errorf("FloatPrecision=2 should truncate 3.14159 to 3.14, got %q", result)
		}
	})

	t.Run("FloatPrecision0", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.FloatPrecision = 0
		cfg.FloatTruncate = true

		result, err := EncodeWithConfig(map[string]any{"val": 3.14159}, cfg)
		if err != nil {
			t.Fatalf("EncodeWithConfig error: %v", err)
		}
		if !strings.Contains(result, "3") || strings.Contains(result, "3.") {
			t.Errorf("FloatPrecision=0 should remove decimal, got %q", result)
		}
	})

	t.Run("FloatPrecision5", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.FloatPrecision = 5
		cfg.FloatTruncate = true

		result, err := EncodeWithConfig(map[string]any{"val": 3.14}, cfg)
		if err != nil {
			t.Fatalf("EncodeWithConfig error: %v", err)
		}
		if !strings.Contains(result, "3.14000") {
			t.Errorf("FloatPrecision=5 should pad with zeros, got %q", result)
		}
	})

	t.Run("FloatPrecisionRoundNotTruncate", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.FloatPrecision = 2
		cfg.FloatTruncate = false

		result, err := EncodeWithConfig(map[string]any{"val": 3.14159}, cfg)
		if err != nil {
			t.Fatalf("EncodeWithConfig error: %v", err)
		}
		if !strings.Contains(result, "3.14") {
			t.Errorf("FloatPrecision=2 (round) should produce 3.14, got %q", result)
		}
	})

	t.Run("NegativePrecisionDisabled", func(t *testing.T) {
		cfg := DefaultConfig()
		// Default FloatPrecision is -1, which means disabled

		result, err := EncodeWithConfig(map[string]any{"val": 3.14159}, cfg)
		if err != nil {
			t.Fatalf("EncodeWithConfig error: %v", err)
		}
		if !strings.Contains(result, "3.14159") {
			t.Errorf("FloatPrecision=-1 should use default formatting, got %q", result)
		}
	})
}

// TestProcessorValuesEqual tests valuesEqual via Processor schema validation
func TestProcessorValuesEqual(t *testing.T) {
	processor, err := New()
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer processor.Close()

	tests := []struct {
		name string
		a, b any
		want bool
	}{
		{"BothNil", nil, nil, true},
		{"NilVsNonNil", nil, 42, false},
		{"NonNilVsNil", 42, nil, false},
		{"SameInt", 42, 42, true},
		{"DifferentInt", 42, 43, false},
		{"SameString", "hello", "hello", true},
		{"DifferentString", "hello", "world", false},
		{"IntVsInt64", 42, int64(42), true},
		{"IntVsFloat64", 42, float64(42), true},
		{"Float64VsInt", float64(42), 42, true},
		{"IntVsFloat32", 42, float32(42), true},
		{"IntVsInt32", 42, int32(42), true},
		{"Float64VsFloat32", float64(3), float32(3), true},
		{"DifferentFloat", 3.14, 2.71, false},
		{"SameBool", true, true, true},
		{"DifferentBool", true, false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := processor.valuesEqual(tt.a, tt.b)
			if got != tt.want {
				t.Errorf("valuesEqual(%v (%T), %v (%T)) = %v, want %v",
					tt.a, tt.a, tt.b, tt.b, got, tt.want)
			}
		})
	}
}

// TestDeepCopySliceBranch verifies deepCopy produces deep (not shallow)
// copies of arrays with nested containers. The copy flows through the
// deepCopySubtree fast path (Get's cache isolation uses this). The
// deepCopySliceWithDepth fallback branch is unreachable for []any — the fast
// path succeeds first — see the GEN-001 dead-code finding in changes.log.
func TestDeepCopySliceBranch(t *testing.T) {
	original := []any{
		1, "two", 3.5, true, nil,
		[]any{4, map[string]any{"k": "v"}},
		map[string]any{"nested": []any{5}},
	}
	copied, err := deepCopy(original)
	if err != nil {
		t.Fatalf("deepCopy error: %v", err)
	}
	arr, ok := copied.([]any)
	if !ok {
		t.Fatalf("deepCopy([]any) returned %T, want []any", copied)
	}
	if len(arr) != len(original) {
		t.Errorf("copied length = %d, want %d", len(arr), len(original))
	}

	// Mutating the copy must not affect the original — deep, not shallow.
	if inner, ok := arr[5].([]any); ok {
		if m, ok := inner[1].(map[string]any); ok {
			m["k"] = "mutated"
		}
	}
	if inner, ok := original[5].([]any); ok {
		if m, ok := inner[1].(map[string]any); ok {
			if m["k"] != "v" {
				t.Errorf("deepCopy is not deep: mutating the copy changed the original (k=%q)", m["k"])
			}
		}
	}
}

// TestDeleteFastPathErrorBranches covers the fast-path error branches of
// Delete (invalid JSON, missing key), reached only when caching is disabled —
// the default config enables the cache, so these branches (and the
// newOperationPathError context they build) were never exercised.
func TestDeleteFastPathErrorBranches(t *testing.T) {
	cfg := DefaultConfig()
	cfg.EnableCache = false
	p, err := New(cfg)
	if err != nil {
		t.Fatalf("New(cfg) error: %v", err)
	}
	defer p.Close()

	if _, err := p.Delete(`{invalid`, "key"); err == nil {
		t.Error("Delete on invalid JSON should fail")
	} else if !errors.Is(err, ErrInvalidJSON) {
		t.Errorf("Delete on invalid JSON: want ErrInvalidJSON, got %v", err)
	}

	if _, err := p.Delete(`{"a":1}`, "missing"); err == nil {
		t.Error("Delete of a missing key should fail")
	} else if !errors.Is(err, ErrPathNotFound) {
		t.Errorf("Delete of a missing key: want ErrPathNotFound, got %v", err)
	}
}

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
		p, err := New(cfg)
		if err != nil {
			t.Fatalf("New() failed: %v", err)
		}
		defer p.Close()

		// operationWindow is the internal max-ops/sec; a small window makes the
		// minimum interval between operations large enough that a second call in
		// immediate succession must be rejected by checkRateLimit (processor_get.go).
		p.metrics.operationWindow = 100 // 100 ops/sec => 10ms minimum interval

		// First call records lastOperationTime and succeeds.
		if _, err := p.Get(`{"a":1}`, "a"); err != nil {
			t.Fatalf("first Get failed: %v", err)
		}

		// Second call fires well within the 10ms window => rate-limited.
		_, err = p.Get(`{"a":1}`, "a")
		if err == nil {
			t.Fatal("second Get should have been rejected by the rate limit, got nil")
		}
		if !strings.Contains(err.Error(), "rate limit") {
			t.Errorf("expected a rate-limit error, got: %v", err)
		}
	})

	t.Run("disabled by default", func(t *testing.T) {
		p, err := New(DefaultConfig())
		if err != nil {
			t.Fatalf("New() failed: %v", err)
		}
		defer p.Close()

		// operationWindow defaults to 0 => rate limiting disabled; rapid calls are fine.
		for range 5 {
			if _, err := p.Get(`{"a":1}`, "a"); err != nil {
				t.Fatalf("Get failed with rate limiting disabled: %v", err)
			}
		}
	})
}

// ============================================================================
// operation_set.go: array auto-extension & extraction-set paths (low coverage)
// Confirmed behaviors probed via the public Set API.
// ============================================================================

func TestSetArrayExtension_Coverage(t *testing.T) {
	// Auto-extension on out-of-bounds index: the array grows with nulls to fit.
	t.Run("auto extend with nulls", func(t *testing.T) {
		result, err := Set(`{"arr":[1,2]}`, "arr[5]", 9)
		if err != nil {
			t.Fatalf("Set failed: %v", err)
		}
		assertJSONEqual(t, `{"arr":[1,2,null,null,null,9]}`, result)
	})

	t.Run("in bounds append via index", func(t *testing.T) {
		result, err := Set(`{"arr":[1,2]}`, "arr[2]", 3)
		if err != nil {
			t.Fatalf("Set failed: %v", err)
		}
		assertJSONEqual(t, `{"arr":[1,2,3]}`, result)
	})

	t.Run("slice set auto extends", func(t *testing.T) {
		result, err := Set(`{"arr":[1,2]}`, "arr[0:5]", 9)
		if err != nil {
			t.Fatalf("Set failed: %v", err)
		}
		assertJSONEqual(t, `{"arr":[9,9,9,9,9]}`, result)
	})

	t.Run("negative index sets last element", func(t *testing.T) {
		result, err := Set(`{"arr":[1,2]}`, "arr[-1]", 9)
		if err != nil {
			t.Fatalf("Set failed: %v", err)
		}
		assertJSONEqual(t, `{"arr":[1,9]}`, result)
	})

	t.Run("create nested path", func(t *testing.T) {
		result, err := Set(`{}`, "a.b.c", 1)
		if err != nil {
			t.Fatalf("Set failed: %v", err)
		}
		assertJSONEqual(t, `{"a":{"b":{"c":1}}}`, result)
	})
}

// TestSetExtraction_Coverage exercises the extraction-Set code paths
// (setValueForExtract, setValueForArrayExtract, setValueForArrayExtractFlat,
// navigateToExtraction) via {field} extraction syntax on objects and arrays.
func TestSetExtraction_Coverage(t *testing.T) {
	t.Run("extract set on single object", func(t *testing.T) {
		result, err := Set(`{"u":{}}`, "u{name}", "x")
		if err != nil {
			t.Fatalf("Set failed: %v", err)
		}
		assertJSONEqual(t, `{"u":{"name":"x"}}`, result)
	})

	t.Run("extract set on array of objects", func(t *testing.T) {
		result, err := Set(`{"us":[{"a":1},{"a":2}]}`, "us{a}", 99)
		if err != nil {
			t.Fatalf("Set failed: %v", err)
		}
		assertJSONEqual(t, `{"us":[{"a":99},{"a":99}]}`, result)
	})

	t.Run("flat extract set", func(t *testing.T) {
		result, err := Set(`{"us":[{"t":[1]},{"t":[2]}]}`, "us{flat:t}", 99)
		if err != nil {
			t.Fatalf("Set failed: %v", err)
		}
		assertJSONEqual(t, `{"us":[{"t":99},{"t":99}]}`, result)
	})
}

// ============================================================================
// operation_array.go: multi-field / post-extraction Get paths (low coverage)
// ============================================================================

func TestGetExtraction_Coverage(t *testing.T) {
	t.Run("extract then slice", func(t *testing.T) {
		got, err := Get(`{"us":[{"id":1},{"id":2},{"id":3}]}`, "us{id}[0:2]")
		if err != nil {
			t.Fatalf("Get failed: %v", err)
		}
		arr, ok := got.([]any)
		if !ok {
			t.Fatalf("expected []any, got %T (%v)", got, got)
		}
		if len(arr) != 2 || arr[0] != 1.0 || arr[1] != 2.0 {
			t.Errorf("extract-then-slice = %v, want [1 2]", got)
		}
	})

	t.Run("multi-field extract on object", func(t *testing.T) {
		got, err := Get(`{"u":{"id":1,"name":"n","email":"e"}}`, "u{id,name,email}")
		if err != nil {
			t.Fatalf("Get failed: %v", err)
		}
		m, ok := got.(map[string]any)
		if !ok {
			t.Fatalf("expected map, got %T (%v)", got, got)
		}
		if m["id"] != 1.0 || m["name"] != "n" || m["email"] != "e" {
			t.Errorf("multi-field extract = %v, want id/name/email", got)
		}
	})

	t.Run("extract on array yields array of maps", func(t *testing.T) {
		got, err := Get(`{"us":[{"id":1,"name":"a"},{"id":2,"name":"b"}]}`, "us{id,name}")
		if err != nil {
			t.Fatalf("Get failed: %v", err)
		}
		arr, ok := got.([]any)
		if !ok || len(arr) != 2 {
			t.Fatalf("expected 2-element []any, got %T (%v)", got, got)
		}
		first, ok := arr[0].(map[string]any)
		if !ok || first["id"] != 1.0 || first["name"] != "a" {
			t.Errorf("first extracted map = %v, want id=1 name=a", arr[0])
		}
	})
}

// ============================================================================
// White-box coverage of operation_set.go / operation_array.go handlers that
// the PUBLIC Get/Set API bypasses.
//
// The library ships TWO parallel path engines: path.go + operation_*.go, and
// recursive.go. The public Get/Set default to recursive.go, so the
// extraction handlers in operation_set.go (setValueForExtract family) and
// operation_array.go (handleMultiFieldExtraction, handleStructAccess) are
// never reached through the public API. They are pure handler functions, so
// calling them directly — exactly as core_test.go does for handleExtraction —
// is the only way to cover them.
// ============================================================================

func TestSetExtractHandlers_Coverage(t *testing.T) {
	p, err := New()
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer p.Close()

	t.Run("setValueForExtract on object", func(t *testing.T) {
		obj := map[string]any{"a": 1}
		seg := internal.PathSegment{Type: internal.ExtractSegment, Key: "name"}
		if err := p.setValueForExtract(obj, seg, "x", true); err != nil {
			t.Fatalf("setValueForExtract: %v", err)
		}
		if obj["name"] != "x" {
			t.Errorf("field not set: %v", obj)
		}
	})

	t.Run("setValueForExtract on array (non-flat)", func(t *testing.T) {
		arr := []any{map[string]any{"a": 1}, "notmap"}
		seg := internal.PathSegment{Type: internal.ExtractSegment, Key: "k"}
		if err := p.setValueForExtract(arr, seg, 9, true); err != nil {
			t.Fatalf("setValueForExtract: %v", err)
		}
		// both elements now carry k=9; the non-map element is promoted to a map
		first, ok := arr[0].(map[string]any)
		if !ok || first["k"] != 9 {
			t.Errorf("first element not set: %v", arr[0])
		}
		second, ok := arr[1].(map[string]any)
		if !ok || second["k"] != 9 {
			t.Errorf("second element not promoted: %v", arr[1])
		}
	})

	t.Run("setValueForExtract on array (flat)", func(t *testing.T) {
		arr := []any{map[string]any{"tags": []any{"a"}}}
		seg := internal.PathSegment{Type: internal.ExtractSegment, Key: "tags", Flags: internal.FlagIsFlat}
		if err := p.setValueForExtract(arr, seg, "b", true); err != nil {
			t.Fatalf("setValueForExtract flat: %v", err)
		}
		got := arr[0].(map[string]any)["tags"].([]any)
		if len(got) != 2 || got[0] != "a" || got[1] != "b" {
			t.Errorf("flat extract set failed: %v", got)
		}
	})

	t.Run("setValueForExtract rejects non-container", func(t *testing.T) {
		seg := internal.PathSegment{Type: internal.ExtractSegment, Key: "name"}
		if err := p.setValueForExtract(42, seg, "x", true); err == nil {
			t.Error("expected error for extraction set on a number")
		}
	})

	t.Run("setValueForExtract rejects empty key", func(t *testing.T) {
		seg := internal.PathSegment{Type: internal.ExtractSegment, Key: ""}
		if err := p.setValueForExtract(map[string]any{}, seg, "x", true); err == nil {
			t.Error("expected error for empty extraction key")
		}
	})

	t.Run("navigateToExtraction on array returns current", func(t *testing.T) {
		arr := []any{1, 2}
		seg := internal.PathSegment{Type: internal.ExtractSegment, Key: "a"}
		got, err := p.navigateToExtraction(arr, seg, true, nil, 0)
		if err != nil {
			t.Fatalf("navigateToExtraction: %v", err)
		}
		// Array extraction is delegated to distributed operations: the current
		// array is returned unchanged (slices are not comparable, so check by
		// identity of contents).
		out, ok := got.([]any)
		if !ok || len(out) != 2 || out[0] != 1 || out[1] != 2 {
			t.Errorf("array navigation should return current unchanged, got %v", got)
		}
	})

	t.Run("navigateToExtraction on missing field with createPaths", func(t *testing.T) {
		obj := map[string]any{}
		segs := []internal.PathSegment{{Type: internal.ExtractSegment, Key: "name"}}
		// currentIndex is the last segment => createContainerForNextSegment
		// returns nil (the slot will be filled by the caller's value).
		got, err := p.navigateToExtraction(obj, segs[0], true, segs, 0)
		if err != nil {
			t.Fatalf("navigateToExtraction: %v", err)
		}
		if got != nil {
			t.Errorf("last-segment container should be nil, got %v", got)
		}
		if _, ok := obj["name"]; !ok {
			t.Errorf("missing field should be created: %v", obj)
		}
	})

	t.Run("navigateToExtraction missing field without createPaths errors", func(t *testing.T) {
		seg := internal.PathSegment{Type: internal.ExtractSegment, Key: "nope"}
		if _, err := p.navigateToExtraction(map[string]any{}, seg, false, nil, 0); err == nil {
			t.Error("expected error for missing field without createPaths")
		}
	})
}

// TestArrayExtractHandlers_Coverage directly exercises operation_array.go's
// multi-field extraction and struct-access handlers (bypassed by the public
// recursive engine).
func TestArrayExtractHandlers_Coverage(t *testing.T) {
	p, err := New()
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer p.Close()

	t.Run("handleMultiFieldExtraction on object", func(t *testing.T) {
		data := map[string]any{"id": 1, "name": "n", "extra": "x"}
		got, err := p.handleMultiFieldExtraction(data, "id,name", false)
		if err != nil {
			t.Fatalf("handleMultiFieldExtraction: %v", err)
		}
		m, ok := got.(map[string]any)
		if !ok {
			t.Fatalf("expected map, got %T (%v)", got, got)
		}
		if m["id"] != 1 || m["name"] != "n" {
			t.Errorf("multi-field object extract = %v", got)
		}
		if _, ok := m["extra"]; ok {
			t.Errorf("unrequested field leaked into result: %v", m)
		}
	})

	t.Run("handleMultiFieldExtraction on array (flat and non-flat)", func(t *testing.T) {
		data := []any{
			map[string]any{"id": 1, "name": "a"},
			map[string]any{"id": 2, "name": "b"},
		}
		// Non-flat: each item becomes a sub-map.
		got, err := p.handleMultiFieldExtraction(data, "id,name", false)
		if err != nil {
			t.Fatalf("handleMultiFieldExtraction: %v", err)
		}
		if arr, ok := got.([]any); !ok || len(arr) != 2 {
			t.Errorf("non-flat array extract = %v", got)
		}
		// Flat flag exercises the flattenValue branch for completeness.
		if _, err := p.handleMultiFieldExtraction(data, "id,name", true); err != nil {
			t.Fatalf("handleMultiFieldExtraction flat: %v", err)
		}
	})

	t.Run("handleStructAccess direct, case-insensitive, nil, pointer", func(t *testing.T) {
		type sample struct {
			Name string
			Age  int
		}
		s := sample{Name: "x", Age: 5}

		if v := p.handleStructAccess(s, "Name"); v != "x" {
			t.Errorf("direct field = %v, want x", v)
		}
		if v := p.handleStructAccess(s, "age"); v != 5 {
			t.Errorf("case-insensitive field = %v, want 5", v)
		}
		if v := p.handleStructAccess(s, "missing"); v != nil {
			t.Errorf("missing field = %v, want nil", v)
		}
		if v := p.handleStructAccess(nil, "x"); v != nil {
			t.Errorf("nil data = %v, want nil", v)
		}
		if v := p.handleStructAccess(42, "x"); v != nil {
			t.Errorf("non-struct = %v, want nil", v)
		}
		if v := p.handleStructAccess((*sample)(nil), "x"); v != nil {
			t.Errorf("nil pointer = %v, want nil", v)
		}
		if v := p.handleStructAccess(&sample{Name: "p"}, "Name"); v != "p" {
			t.Errorf("non-nil pointer field = %v, want p", v)
		}
	})
}
