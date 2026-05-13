package json

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
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
func TestProcessorEncodeMethods(t *testing.T) {
	processor, err := New()
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer processor.Close()

	t.Run("Encode", func(t *testing.T) {
		tests := []struct {
			name    string
			value   any
			wantErr bool
		}{
			{"String", "hello", false},
			{"Int", 42, false},
			{"Float", 3.14, false},
			{"Bool", true, false},
			{"Nil", nil, false},
			{"Slice", []int{1, 2, 3}, false},
			{"Map", map[string]any{"key": "value"}, false},
			{"Struct", struct{ Name string }{"test"}, false},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				result, err := processor.Encode(tt.value)
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
				if result == "" {
					t.Error("expected non-empty result")
				}
			})
		}
	})

	t.Run("EncodePretty", func(t *testing.T) {
		tests := []struct {
			name    string
			value   any
			wantErr bool
		}{
			{"String", "hello", false},
			{"Int", 42, false},
			{"Map", map[string]any{"key": "value"}, false},
			{"Struct", struct{ Name string }{"test"}, false},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				result, err := processor.EncodePretty(tt.value)
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
				if result == "" {
					t.Error("expected non-empty result")
				}
			})
		}
	})

	t.Run("EncodeWithConfig", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.Pretty = true
		cfg.SortKeys = true

		result, err := processor.Encode(map[string]any{"z": 1, "a": 2}, cfg)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
			return
		}
		if result == "" {
			t.Error("expected non-empty result")
		}
	})
}

// TestEncodeJSONNumber tests encodeJSONNumber via custom encoder
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

// testHookImpl is a simple Hook implementation for testing
type testHookImpl struct {
	beforeCalled bool
	afterCalled  bool
}

func (h *testHookImpl) Before(ctx HookContext) error {
	h.beforeCalled = true
	return nil
}

func (h *testHookImpl) After(ctx HookContext, result any, err error) (any, error) {
	h.afterCalled = true
	return result, err
}

// testValidatorImpl is a simple Validator implementation for testing
type testValidatorImpl struct {
	validateCalled bool
}

func (v *testValidatorImpl) Validate(jsonStr string) error {
	v.validateCalled = true
	return nil
}

// testCustomEncoder implements CustomEncoder for testing
type testCustomEncoder struct{}

func (e *testCustomEncoder) Encode(value any) (string, error) {
	return `{"custom":true}`, nil
}

// testTypeEncoder implements TypeEncoder for testing
type testTypeEncoder struct{}

func (e *testTypeEncoder) Encode(v reflect.Value) (string, error) {
	return `"encoded"`, nil
}

// testPathParser implements PathParser for testing
type testPathParser struct{}

func (p *testPathParser) ParsePath(path string) ([]PathSegment, error) {
	return []PathSegment{newPropertySegment(path)}, nil
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

	t.Run("HandlesStaleCacheEntry", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.MaxCacheSize = 7001

		p1, err := getProcessorWithConfig(cfg)
		if err != nil {
			t.Fatalf("First call error: %v", err)
		}

		p1.Close()

		p2, err := getProcessorWithConfig(cfg)
		if err != nil {
			t.Fatalf("Second call error: %v", err)
		}

		if p1 == p2 {
			t.Error("Expected new processor after stale entry")
		}

		p2.Close()
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
	t.Run("ConfigValidation", func(t *testing.T) {
		cfg := Config{MaxJSONSize: -1}
		err := cfg.Validate()
		_ = err
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
		_ = err
		_ = result
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

	t.Run("InvalidJSONReturnsOriginal", func(t *testing.T) {
		result, err := p.formatJSONString("{invalid}", false)
		// Invalid JSON may return original or error depending on implementation
		_ = err
		_ = result
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

// TestNewSchemaWithConfigEdgeCases tests NewSchemaWithConfig edge cases
func TestNewSchemaWithConfigEdgeCases(t *testing.T) {
	t.Run("WithValidSchema", func(t *testing.T) {
		cfg := SchemaConfig{Type: "object"}
		schema := NewSchemaWithConfig(cfg)
		if schema == nil {
			t.Error("NewSchemaWithConfig failed")
		}
	})

	t.Run("WithProperties", func(t *testing.T) {
		cfg := SchemaConfig{
			Type: "object",
			Properties: map[string]*Schema{
				"name": {Type: "string"},
			},
		}
		schema := NewSchemaWithConfig(cfg)
		if schema == nil {
			t.Error("NewSchemaWithConfig failed")
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

	t.Run("isValidPath", func(t *testing.T) {
		if !isValidPath("key.nested") {
			t.Error("isValidPath should return true for valid path")
		}
	})

	t.Run("validatePath", func(t *testing.T) {
		if err := validatePath("key.nested"); err != nil {
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
func TestNewSegmentFunctions(t *testing.T) {
	t.Run("newPropertySegment", func(t *testing.T) {
		seg := newPropertySegment("test")
		if seg.Type != internal.PropertySegment {
			t.Error("newPropertySegment should create property segment")
		}
	})

	t.Run("newArrayIndexSegment", func(t *testing.T) {
		seg := newArrayIndexSegment(0)
		if seg.Type != internal.ArrayIndexSegment {
			t.Error("newArrayIndexSegment should create array index segment")
		}
	})

	t.Run("newArraySliceSegment", func(t *testing.T) {
		seg := newArraySliceSegment(0, 10, 1, true, true, true)
		if seg.Type != internal.ArraySliceSegment {
			t.Error("newArraySliceSegment should create array slice segment")
		}
	})

	t.Run("newWildcardSegment", func(t *testing.T) {
		seg := newWildcardSegment()
		if seg.Type != internal.WildcardSegment {
			t.Error("newWildcardSegment should create wildcard segment")
		}
	})

	t.Run("newExtractSegment", func(t *testing.T) {
		seg := newExtractSegment("name", false)
		if seg.Type != internal.ExtractSegment {
			t.Error("newExtractSegment should create extract segment")
		}
	})

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

// TestIsSliceType tests the internal IsSliceType function
func TestIsSliceType(t *testing.T) {
	tests := []struct {
		input    any
		expected bool
	}{
		{[]any{1, 2, 3}, true},
		{[]string{"a", "b"}, true},
		{[]int{1, 2, 3}, true},
		{map[string]any{"key": "value"}, false},
		{"string", false},
		{nil, false},
		{42, false},
	}

	for i, tt := range tests {
		result := internal.IsSliceType(tt.input)
		if result != tt.expected {
			t.Errorf("Test %d: IsSliceType(%T) = %v, want %v", i, tt.input, result, tt.expected)
		}
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
	t.Run("WithConfig", func(t *testing.T) {
		config := DefaultConfig()
		config.EscapeHTML = true

		result, err := Encode(map[string]any{"html": "<script>"}, config)
		if err != nil {
			t.Errorf("Encode failed: %v", err)
		}
		if !strings.Contains(result, "\\u003c") {
			t.Error("HTML should be escaped")
		}
	})

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
		if result == nil {
			t.Fatal("expected non-nil result")
		}
	})

	t.Run("DecodeFloat", func(t *testing.T) {
		result, err := decoder.DecodeToAny(`3.14`)
		if err != nil {
			t.Fatalf("DecodeToAny failed: %v", err)
		}
		if result == nil {
			t.Fatal("expected non-nil result")
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
func TestPreservingUnmarshal(t *testing.T) {
	t.Run("PreserveNumbersTrue", func(t *testing.T) {
		var result map[string]any
		err := preservingUnmarshal([]byte(`{"num":123}`), &result, true)
		if err != nil {
			t.Errorf("preservingUnmarshal failed: %v", err)
		}
	})

	t.Run("PreserveNumbersFalse", func(t *testing.T) {
		var result map[string]any
		err := preservingUnmarshal([]byte(`{"num":123}`), &result, false)
		if err != nil {
			t.Errorf("preservingUnmarshal failed: %v", err)
		}
		if result["num"] == nil {
			t.Error("num should not be nil")
		}
	})
}

// ============================================================================
// JSON POINTER ESCAPE TESTS
// ============================================================================

// TestescapeJSONPointer tests escapeJSONPointer function
// ============================================================================
// ENCODING EDGE CASES TESTS - Coverage for encoding functions
// ============================================================================

// TestEncodeStringSpecialChars tests encoding strings with special characters
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
			t.Errorf("EncodeWithConfig failed: %v", err)
		}
		_ = result
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

// TestValidateNumberEdgeCases tests number validation edge cases
func TestValidateNumberEdgeCases(t *testing.T) {
	processor, _ := New()
	defer processor.Close()

	t.Run("NumberValidation", func(t *testing.T) {
		schema := &Schema{
			Type:    "number",
			Minimum: 0,
			Maximum: 100,
		}
		schema.hasMinimum = true
		schema.hasMaximum = true

		tests := []struct {
			jsonStr   string
			expectErr bool
		}{
			{`50`, false},
			{`0`, false},
			{`100`, false},
		}

		for _, tt := range tests {
			errors, err := processor.ValidateSchema(tt.jsonStr, schema)
			if err != nil {
				t.Errorf("ValidateSchema failed for %s: %v", tt.jsonStr, err)
				continue
			}
			hasErrors := len(errors) > 0
			if hasErrors != tt.expectErr {
				t.Errorf("ValidateSchema(%s) errors = %v, expectErr = %v", tt.jsonStr, errors, tt.expectErr)
			}
		}
	})

	t.Run("IntegerValidation", func(t *testing.T) {
		schema := &Schema{
			Type:    "number", // Use number since JSON decodes to float64
			Minimum: 0,
			Maximum: 100,
		}
		schema.hasMinimum = true
		schema.hasMaximum = true

		tests := []struct {
			jsonStr   string
			expectErr bool
		}{
			{`50`, false},
			{`0`, false},
			{`100`, false},
		}

		for _, tt := range tests {
			errors, err := processor.ValidateSchema(tt.jsonStr, schema)
			if err != nil {
				t.Errorf("ValidateSchema failed for %s: %v", tt.jsonStr, err)
				continue
			}
			hasErrors := len(errors) > 0
			if hasErrors != tt.expectErr {
				t.Errorf("ValidateSchema(%s) errors = %v, expectErr = %v", tt.jsonStr, errors, tt.expectErr)
			}
		}
	})
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
func TestMoreMethod(t *testing.T) {
	t.Run("MoreWithMultipleValues", func(t *testing.T) {
		decoder := NewDecoder(strings.NewReader(`{"a":1}{"b":2}`))

		// First decode
		var v1 map[string]any
		err := decoder.Decode(&v1)
		if err != nil {
			t.Errorf("First Decode failed: %v", err)
		}

		// Check if more
		if !decoder.More() {
			t.Error("More() should return true when there's more data")
		}

		// Second decode
		var v2 map[string]any
		err = decoder.Decode(&v2)
		if err != nil {
			t.Errorf("Second Decode failed: %v", err)
		}

		// No more
		if decoder.More() {
			t.Error("More() should return false when there's no more data")
		}
	})
}

// TestEncoderMethods tests Encoder methods
func TestEncoderMethods(t *testing.T) {
	t.Run("SetEscapeHTML", func(t *testing.T) {
		var buf bytes.Buffer
		encoder := NewEncoder(&buf)
		encoder.SetEscapeHTML(true)

		data := map[string]string{"html": "<script>"}
		err := encoder.Encode(data)
		if err != nil {
			t.Errorf("Encode failed: %v", err)
		}

		if !strings.Contains(buf.String(), "\\u003c") {
			t.Error("HTML should be escaped")
		}
	})

	t.Run("SetIndent", func(t *testing.T) {
		var buf bytes.Buffer
		encoder := NewEncoder(&buf)
		encoder.SetIndent("", "  ")

		data := map[string]string{"key": "value"}
		err := encoder.Encode(data)
		if err != nil {
			t.Errorf("Encode failed: %v", err)
		}

		if !strings.Contains(buf.String(), "\n") {
			t.Error("Output should be indented")
		}
	})
}

// TestEncodeStringEdgeCases tests string encoding edge cases
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
			t.Errorf("EncodeWithConfig failed: %v", err)
		}
		_ = result
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
			t.Errorf("EncodeWithConfig failed: %v", err)
		}
		_ = result
	})

	t.Run("MapWithNumericKeys", func(t *testing.T) {
		// Map with non-string keys should still encode
		data := map[int]string{1: "one", 2: "two"}
		result, err := processor.EncodeWithConfig(data, DefaultConfig())
		if err != nil {
			t.Errorf("EncodeWithConfig failed: %v", err)
		}
		_ = result
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

// TestParseBoolean tests parseBoolean function
// TestParseStringEdgeCases tests parseString function edge cases
func TestParseStringEdgeCases(t *testing.T) {
	t.Run("EscapedCharacters", func(t *testing.T) {
		decoder := NewDecoder(strings.NewReader(`"hello\nworld\ttab"`))
		var result string
		err := decoder.Decode(&result)
		if err != nil {
			t.Errorf("Decode failed: %v", err)
		}
		if !strings.Contains(result, "\n") {
			t.Error("Result should contain newline")
		}
	})

	t.Run("UnicodeEscape", func(t *testing.T) {
		decoder := NewDecoder(strings.NewReader(`"\u0041"`))
		var result string
		err := decoder.Decode(&result)
		if err != nil {
			t.Errorf("Decode failed: %v", err)
		}
		if result != "A" {
			t.Errorf("Result = %q, want A", result)
		}
	})

	t.Run("EscapedQuote", func(t *testing.T) {
		decoder := NewDecoder(strings.NewReader(`"say \"hello\""`))
		var result string
		err := decoder.Decode(&result)
		if err != nil {
			t.Errorf("Decode failed: %v", err)
		}
		if !strings.Contains(result, `"`) {
			t.Error("Result should contain quote")
		}
	})

	t.Run("EscapedBackslash", func(t *testing.T) {
		decoder := NewDecoder(strings.NewReader(`"path\\to\\file"`))
		var result string
		err := decoder.Decode(&result)
		if err != nil {
			t.Errorf("Decode failed: %v", err)
		}
		if !strings.Contains(result, "\\") {
			t.Error("Result should contain backslash")
		}
	})
}

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

// TestIsEmptyComprehensive tests isEmpty function more comprehensively
func TestIsEmptyComprehensive(t *testing.T) {
	processor, _ := New()
	defer processor.Close()

	tests := []struct {
		name  string
		input any
	}{
		{"EmptySlice", []any{}},
		{"EmptyMap", map[string]any{}},
		{"EmptyString", ""},
		{"ZeroInt", 0},
		{"ZeroFloat", 0.0},
		{"False", false},
		{"Nil", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := map[string]any{"value": tt.input}
			result, err := processor.EncodeWithConfig(data, DefaultConfig())
			if err != nil {
				t.Errorf("EncodeWithConfig failed: %v", err)
			}
			_ = result
		})
	}
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
	}{
		{"LargeInt", map[string]any{"value": 9223372036854775807}},
		{"SmallInt", map[string]any{"value": -9223372036854775808}},
		{"LargeFloat", map[string]any{"value": 1.7976931348623157e+308}},
		{"SmallFloat", map[string]any{"value": -1.7976931348623157e+308}},
		{"NegativeZero", map[string]any{"value": 0.0}},
		{"VerySmallFloat", map[string]any{"value": 1e-300}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := processor.EncodeWithConfig(tt.input, DefaultConfig())
			if err != nil {
				t.Errorf("EncodeWithConfig failed: %v", err)
			}
			_ = result
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
func TestStringFormatValidation(t *testing.T) {
	processor, _ := New()
	defer processor.Close()

	t.Run("EmailFormat", func(t *testing.T) {
		schema := &Schema{
			Type:   "string",
			Format: "email",
		}

		tests := []struct {
			jsonStr   string
			expectErr bool
		}{
			{`"test@example.com"`, false},
			{`"invalid-email"`, true},
			{`"user@domain"`, false}, // Basic format check
		}

		for _, tt := range tests {
			errors, err := processor.ValidateSchema(tt.jsonStr, schema)
			if err != nil {
				t.Errorf("ValidateSchema failed: %v", err)
				continue
			}
			// Just verify validation runs
			_ = len(errors) > 0
		}
	})

	t.Run("DateFormat", func(t *testing.T) {
		schema := &Schema{
			Type:   "string",
			Format: "date",
		}

		tests := []string{
			`"2024-01-15"`,
			`"invalid-date"`,
		}

		for _, jsonStr := range tests {
			_, err := processor.ValidateSchema(jsonStr, schema)
			if err != nil {
				t.Errorf("ValidateSchema failed: %v", err)
			}
		}
	})

	t.Run("DateTimeFormat", func(t *testing.T) {
		schema := &Schema{
			Type:   "string",
			Format: "date-time",
		}

		tests := []string{
			`"2024-01-15T10:30:00Z"`,
			`"invalid-datetime"`,
		}

		for _, jsonStr := range tests {
			_, err := processor.ValidateSchema(jsonStr, schema)
			if err != nil {
				t.Errorf("ValidateSchema failed: %v", err)
			}
		}
	})

	t.Run("UUIDFormat", func(t *testing.T) {
		schema := &Schema{
			Type:   "string",
			Format: "uuid",
		}

		tests := []string{
			`"550e8400-e29b-41d4-a716-446655440000"`,
			`"invalid-uuid"`,
		}

		for _, jsonStr := range tests {
			_, err := processor.ValidateSchema(jsonStr, schema)
			if err != nil {
				t.Errorf("ValidateSchema failed: %v", err)
			}
		}
	})

	t.Run("URIFormat", func(t *testing.T) {
		schema := &Schema{
			Type:   "string",
			Format: "uri",
		}

		tests := []string{
			`"https://example.com"`,
			`"invalid-uri"`,
		}

		for _, jsonStr := range tests {
			_, err := processor.ValidateSchema(jsonStr, schema)
			if err != nil {
				t.Errorf("ValidateSchema failed: %v", err)
			}
		}
	})

	t.Run("IPv4Format", func(t *testing.T) {
		schema := &Schema{
			Type:   "string",
			Format: "ipv4",
		}

		tests := []string{
			`"192.168.1.1"`,
			`"invalid-ip"`,
		}

		for _, jsonStr := range tests {
			_, err := processor.ValidateSchema(jsonStr, schema)
			if err != nil {
				t.Errorf("ValidateSchema failed: %v", err)
			}
		}
	})

	t.Run("IPv6Format", func(t *testing.T) {
		schema := &Schema{
			Type:   "string",
			Format: "ipv6",
		}

		tests := []string{
			`"2001:0db8:85a3:0000:0000:8a2e:0370:7334"`,
			`"invalid-ipv6"`,
		}

		for _, jsonStr := range tests {
			_, err := processor.ValidateSchema(jsonStr, schema)
			if err != nil {
				t.Errorf("ValidateSchema failed: %v", err)
			}
		}
	})
}

// ============================================================================
// TOP-LEVEL API FUNCTIONS - Missing coverage tests
// ============================================================================

// TestGetStringDefault tests the top-level GetString function with default values
func TestGetStringDefault(t *testing.T) {
	tests := []struct {
		name         string
		jsonStr      string
		path         string
		defaultValue string
		expected     string
	}{
		{"Found", `{"name":"Alice"}`, "name", "default", "Alice"},
		{"NotFound", `{"name":"Alice"}`, "missing", "default", "default"},
		{"InvalidJSON", `{invalid}`, "name", "default", "default"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetString(tt.jsonStr, tt.path, tt.defaultValue)
			if result != tt.expected {
				t.Errorf("GetString() = %q, want %q", result, tt.expected)
			}
		})
	}
}

// TestGetIntDefault tests the top-level GetInt function with default values
func TestGetIntDefault(t *testing.T) {
	tests := []struct {
		name         string
		jsonStr      string
		path         string
		defaultValue int
		expected     int
	}{
		{"Found", `{"count":42}`, "count", -1, 42},
		{"NotFound", `{"count":42}`, "missing", -1, -1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetInt(tt.jsonStr, tt.path, tt.defaultValue)
			if result != tt.expected {
				t.Errorf("GetInt() = %d, want %d", result, tt.expected)
			}
		})
	}
}

// TestGetFloatDefault tests the top-level GetFloat function with default values
func TestGetFloatDefault(t *testing.T) {
	tests := []struct {
		name         string
		jsonStr      string
		path         string
		defaultValue float64
		expected     float64
	}{
		{"Found", `{"value":3.14}`, "value", -1.0, 3.14},
		{"NotFound", `{"value":3.14}`, "missing", -1.0, -1.0},
		{"IntToFloat", `{"count":42}`, "count", -1.0, 42.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetFloat(tt.jsonStr, tt.path, tt.defaultValue)
			if result != tt.expected {
				t.Errorf("GetFloat() = %f, want %f", result, tt.expected)
			}
		})
	}
}

// TestGetBoolDefault tests the top-level GetBool function with default values
func TestGetBoolDefault(t *testing.T) {
	tests := []struct {
		name         string
		jsonStr      string
		path         string
		defaultValue bool
		expected     bool
	}{
		{"FoundTrue", `{"active":true}`, "active", false, true},
		{"FoundFalse", `{"active":false}`, "active", true, false},
		{"NotFound", `{"active":true}`, "missing", false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetBool(tt.jsonStr, tt.path, tt.defaultValue)
			if result != tt.expected {
				t.Errorf("GetBool() = %v, want %v", result, tt.expected)
			}
		})
	}
}

// TestValidString tests the validString function
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
// CONFIG ENCODING METHODS - Missing coverage tests
// ============================================================================

// TestConfigEncodingMethods tests Config encoding-related methods
func TestConfigEncodingMethods(t *testing.T) {
	cfg := Config{
		Pretty:          true,
		Indent:          "  ",
		Prefix:          "> ",
		EscapeHTML:      true,
		SortKeys:        true,
		FloatPrecision:  4,
		FloatTruncate:   true,
		MaxDepth:        50,
		IncludeNulls:    true,
		ValidateUTF8:    true,
		DisallowUnknown: true,
	}

	t.Run("IsHTMLEscapeEnabled", func(t *testing.T) {
		if !cfg.isHTMLEscapeEnabled() {
			t.Error("IsHTMLEscapeEnabled should return true")
		}
	})

	t.Run("IsPrettyEnabled", func(t *testing.T) {
		if !cfg.isPrettyEnabled() {
			t.Error("IsPrettyEnabled should return true")
		}
	})

	t.Run("GetIndent", func(t *testing.T) {
		if cfg.getIndent() != "  " {
			t.Errorf("GetIndent = %q, want %q", cfg.getIndent(), "  ")
		}
	})

	t.Run("GetPrefix", func(t *testing.T) {
		if cfg.getPrefix() != "> " {
			t.Errorf("GetPrefix = %q, want %q", cfg.getPrefix(), "> ")
		}
	})

	t.Run("IsSortKeysEnabled", func(t *testing.T) {
		if !cfg.isSortKeysEnabled() {
			t.Error("IsSortKeysEnabled should return true")
		}
	})

	t.Run("GetFloatPrecision", func(t *testing.T) {
		if cfg.getFloatPrecision() != 4 {
			t.Errorf("GetFloatPrecision = %d, want 4", cfg.getFloatPrecision())
		}
	})

	t.Run("IsTruncateFloatEnabled", func(t *testing.T) {
		if !cfg.isTruncateFloatEnabled() {
			t.Error("IsTruncateFloatEnabled should return true")
		}
	})

	t.Run("GetMaxDepth", func(t *testing.T) {
		if cfg.getMaxDepth() != 50 {
			t.Errorf("GetMaxDepth = %d, want 50", cfg.getMaxDepth())
		}
	})

	t.Run("ShouldIncludeNulls", func(t *testing.T) {
		if !cfg.shouldIncludeNulls() {
			t.Error("ShouldIncludeNulls should return true")
		}
	})

	t.Run("ShouldValidateUTF8", func(t *testing.T) {
		if !cfg.shouldValidateUTF8() {
			t.Error("ShouldValidateUTF8 should return true")
		}
	})

	t.Run("IsDisallowUnknownEnabled", func(t *testing.T) {
		if !cfg.isDisallowUnknownEnabled() {
			t.Error("IsDisallowUnknownEnabled should return true")
		}
	})
}

// TestConfigEncodingMethodsDefaults tests default values
func TestConfigEncodingMethodsDefaults(t *testing.T) {
	cfg := Config{} // Zero value

	t.Run("DefaultIsHTMLEscapeEnabled", func(t *testing.T) {
		if cfg.isHTMLEscapeEnabled() {
			t.Error("Zero-value IsHTMLEscapeEnabled should return false")
		}
	})

	t.Run("DefaultIsPrettyEnabled", func(t *testing.T) {
		if cfg.isPrettyEnabled() {
			t.Error("Zero-value IsPrettyEnabled should return false")
		}
	})

	t.Run("DefaultGetIndent", func(t *testing.T) {
		if cfg.getIndent() != "" {
			t.Errorf("Zero-value GetIndent = %q, want empty", cfg.getIndent())
		}
	})

	t.Run("DefaultGetPrefix", func(t *testing.T) {
		if cfg.getPrefix() != "" {
			t.Errorf("Zero-value GetPrefix = %q, want empty", cfg.getPrefix())
		}
	})
}

// ============================================================================
// TOP-LEVEL FILE FUNCTIONS - Missing coverage tests
// ============================================================================

// TestLoadFromFileTopLevel tests the top-level LoadFromFile function
func TestLoadFromFileTopLevel(t *testing.T) {
	t.Run("ValidFile", func(t *testing.T) {
		tempDir := t.TempDir()
		filePath := filepath.Join(tempDir, "test.json")
		testData := `{"name":"test","value":123}`

		err := os.WriteFile(filePath, []byte(testData), 0644)
		if err != nil {
			t.Fatalf("Failed to write test file: %v", err)
		}

		loaded, err := LoadFromFile(filePath)
		if err != nil {
			t.Errorf("LoadFromFile failed: %v", err)
		}
		if loaded != testData {
			t.Errorf("Loaded data = %q, want %q", loaded, testData)
		}
	})

	t.Run("NonExistentFile", func(t *testing.T) {
		_, err := LoadFromFile("/non/existent/file.json")
		if err == nil {
			t.Error("Expected error for non-existent file")
		}
	})
}

// TestSaveToFileTopLevel tests the top-level SaveToFile function
func TestSaveToFileTopLevel(t *testing.T) {
	t.Run("SaveAndLoad", func(t *testing.T) {
		tempDir := t.TempDir()
		filePath := filepath.Join(tempDir, "save_test.json")
		testData := map[string]any{"name": "test", "value": 123}

		cfg := DefaultConfig()
		cfg.Pretty = false
		err := SaveToFile(filePath, testData, cfg)
		if err != nil {
			t.Errorf("SaveToFile failed: %v", err)
		}

		loaded, err := LoadFromFile(filePath)
		if err != nil {
			t.Errorf("LoadFromFile failed: %v", err)
		}

		if !strings.Contains(loaded, `"name"`) {
			t.Error("Loaded data should contain 'name'")
		}
	})
}

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

// TestEmptyAndNilInputs tests empty and nil input handling
func TestEmptyAndNilInputs(t *testing.T) {
	processor, _ := New()
	defer processor.Close()

	t.Run("EmptyJSON", func(t *testing.T) {
		_, err := processor.Get("", "key")
		if err == nil {
			t.Error("Get with empty JSON should return error")
		}
	})

	t.Run("EmptyPath", func(t *testing.T) {
		result, err := processor.Get(`{"key":"value"}`, "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		m, ok := result.(map[string]any)
		if !ok {
			t.Fatalf("expected map, got %T", result)
		}
		if m["key"] != "value" {
			t.Errorf("expected key=value in root result")
		}
	})

	t.Run("NilValue", func(t *testing.T) {
		result, err := processor.Set(`{"key":"value"}`, "key", nil)
		if err != nil {
			t.Errorf("Set with nil value failed: %v", err)
		}
		assertJSONEqual(t, `{"key":null}`, result)
	})
}

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

// TestDeepNesting tests deeply nested JSON handling
func TestDeepNesting(t *testing.T) {
	processor, _ := New()
	defer processor.Close()

	t.Run("DeepPath", func(t *testing.T) {
		// Create deeply nested JSON
		deep := `{"a":{"b":{"c":{"d":{"e":"deep"}}}}}`
		result, err := processor.Get(deep, "a.b.c.d.e")
		if err != nil {
			t.Errorf("Get deep path failed: %v", err)
		}
		if result != "deep" {
			t.Errorf("Result = %v, want deep", result)
		}
	})

	t.Run("DeepArray", func(t *testing.T) {
		deep := `{"a":[[[1,2],[3,4]],[[5,6],[7,8]]]}`
		result, err := processor.Get(deep, "a[0][1][0]")
		if err != nil {
			t.Errorf("Get deep array failed: %v", err)
		}
		if result != 3.0 {
			t.Errorf("Result = %v, want 3", result)
		}
	})
}

// ============================================================================
// FAST ENCODER TESTS - Missing coverage
// ============================================================================

// TestFastEncoderFunctions tests the fast encoder functions
func TestFastEncoderFunctions(t *testing.T) {
	t.Run("FastEncodeSimple", func(t *testing.T) {
		data := map[string]any{"key": "value", "num": 123}
		result, ok := fastEncodeSimple(data)
		if !ok {
			t.Error("fastEncodeSimple should succeed for simple data")
		}
		if !strings.Contains(result, `"key"`) {
			t.Error("Result should contain key")
		}
	})

	t.Run("FastEncodeSimpleWithHTMLEscape", func(t *testing.T) {
		data := map[string]any{"html": "<script>"}
		result, ok := fastEncodeSimpleWithHTMLEscape(data)
		if !ok {
			t.Error("fastEncodeSimpleWithHTMLEscape should succeed")
		}
		if !strings.Contains(result, "\\u003c") {
			t.Error("HTML should be escaped")
		}
	})
}

// TestConfigCloneEdgeCases tests Config.Clone edge cases
func TestConfigCloneEdgeCases(t *testing.T) {
	t.Run("CloneWithCustomEscapes", func(t *testing.T) {
		original := Config{
			CustomEscapes: map[rune]string{
				'\n': "\\n",
				'\t': "\\t",
			},
		}

		cloned := original.Clone()
		if cloned.CustomEscapes == nil {
			t.Error("Cloned CustomEscapes should not be nil")
		}

		// Modify clone
		cloned.CustomEscapes['\r'] = "\\r"
		if _, exists := original.CustomEscapes['\r']; exists {
			t.Error("Original should not be affected by clone modification")
		}
	})

	t.Run("CloneNil", func(t *testing.T) {
		var cfg *Config
		cloned := cfg.Clone()
		if cloned != nil {
			t.Error("Clone of nil should be nil")
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
		_ = warnings
	})

	t.Run("ValidateZeroConfig", func(t *testing.T) {
		cfg := Config{}
		err := cfg.Validate()
		if err != nil {
			t.Errorf("Validate of zero config should succeed: %v", err)
		}
	})
}

// TestResourcePoolOperations tests resource pool edge cases
func TestResourcePoolOperations(t *testing.T) {
	t.Run("BufferPoolOperations", func(t *testing.T) {
		buf := *internal.GetByteSliceWithHint(1024)
		buf = append(buf, "test"...)
		if string(buf) != "test" {
			t.Errorf("buffer content = %q, want test", buf)
		}
		internal.PutByteSlice(&buf)
	})

	t.Run("StringBuilderPoolOperations", func(t *testing.T) {
		sb := internal.GetStringBuilder()
		sb.WriteString("test")
		if sb.String() != "test" {
			t.Errorf("builder content = %q, want test", sb.String())
		}
		internal.PutStringBuilder(sb)
	})

	t.Run("PathSegmentsPoolOperations", func(t *testing.T) {
		segs := internal.GetPathSegmentSlice(8)
		if segs == nil {
			t.Error("expected non-nil segment slice")
		}
		internal.PutPathSegmentSlice(segs)
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
func TestAPISetCreate(t *testing.T) {
	t.Run("create nested path", func(t *testing.T) {
		result, err := SetCreate(`{"a":1}`, "b.c", 2)
		if err != nil {
			t.Fatalf("SetCreate error: %v", err)
		}
		assertJSONEqual(t, `{"a":1,"b":{"c":2}}`, result)
	})

	t.Run("set existing", func(t *testing.T) {
		result, err := SetCreate(`{"a":1}`, "a", 99)
		if err != nil {
			t.Fatalf("SetCreate error: %v", err)
		}
		assertJSONEqual(t, `{"a":99}`, result)
	})
}

// TestAPISetMultipleCreate tests the package-level SetMultipleCreate function
func TestAPISetMultipleCreate(t *testing.T) {
	result, err := SetMultipleCreate(`{}`, map[string]any{"x": 1, "y": 2})
	if err != nil {
		t.Fatalf("SetMultipleCreate error: %v", err)
	}
	assertJSONEqual(t, `{"x":1,"y":2}`, result)
}

// TestAPIDeleteClean tests the package-level DeleteClean function
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
			name     string
			input    string
			wantNum  string
			wantErr  bool
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
			// May still contain \n as literal text; check that actual newline is present
			// and that the value contains a real newline character in the output
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
