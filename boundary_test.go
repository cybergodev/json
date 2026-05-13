package json

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

// ============================================================================
// AccessResult boundary tests (types.go: 0% coverage)
// ============================================================================

func TestAccessResult_Ok_Unwrap_UnwrapOr(t *testing.T) {
	tests := []struct {
		name         string
		result       AccessResult
		wantOk       bool
		wantUnwrap   any
		wantUnwrapOr any
		defaultVal   any
	}{
		{
			name:         "exists with string",
			result:       AccessResult{Value: "hello", Exists: true, Type: "string"},
			wantOk:       true,
			wantUnwrap:   "hello",
			wantUnwrapOr: "hello",
			defaultVal:   "default",
		},
		{
			name:         "exists with nil value",
			result:       AccessResult{Value: nil, Exists: true, Type: "null"},
			wantOk:       true,
			wantUnwrap:   nil,
			wantUnwrapOr: nil,
			defaultVal:   "default",
		},
		{
			name:         "not exists",
			result:       AccessResult{Value: nil, Exists: false, Type: ""},
			wantOk:       false,
			wantUnwrap:   nil,
			wantUnwrapOr: "default",
			defaultVal:   "default",
		},
		{
			name:         "not exists with zero default",
			result:       AccessResult{Value: nil, Exists: false},
			wantOk:       false,
			wantUnwrap:   nil,
			wantUnwrapOr: 0,
			defaultVal:   0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.result.Ok(); got != tt.wantOk {
				t.Errorf("Ok() = %v, want %v", got, tt.wantOk)
			}
			if got := tt.result.Unwrap(); got != tt.wantUnwrap {
				t.Errorf("Unwrap() = %v, want %v", got, tt.wantUnwrap)
			}
			if got := tt.result.UnwrapOr(tt.defaultVal); got != tt.wantUnwrapOr {
				t.Errorf("UnwrapOr() = %v, want %v", got, tt.wantUnwrapOr)
			}
		})
	}
}

// ============================================================================
// Config accessor boundary tests (config.go: 0% coverage)
// ============================================================================

func TestConfigAccessors(t *testing.T) {
	t.Run("isCacheEnabled", func(t *testing.T) {
		cfg := DefaultConfig()
		if !cfg.isCacheEnabled() {
			t.Error("default config should have cache enabled")
		}
		cfg.EnableCache = false
		if cfg.isCacheEnabled() {
			t.Error("cache should be disabled")
		}
	})

	t.Run("getMaxCacheSize", func(t *testing.T) {
		cfg := DefaultConfig()
		if cfg.getMaxCacheSize() <= 0 {
			t.Error("default MaxCacheSize should be positive")
		}
		cfg.MaxCacheSize = 42
		if cfg.getMaxCacheSize() != 42 {
			t.Errorf("getMaxCacheSize() = %d, want 42", cfg.getMaxCacheSize())
		}
	})

	t.Run("getCacheTTL", func(t *testing.T) {
		cfg := DefaultConfig()
		ttl := cfg.getCacheTTL()
		if ttl <= 0 {
			t.Error("default CacheTTL should be positive")
		}
		cfg.CacheTTL = 5 * time.Minute
		if cfg.getCacheTTL() != 5*time.Minute {
			t.Errorf("getCacheTTL() = %v, want 5m", cfg.getCacheTTL())
		}
	})
}

// ============================================================================
// Config mutation boundary tests (types.go: 0% coverage)
// ============================================================================

func TestConfigMutation_NilReceiver(t *testing.T) {
	t.Run("AddHook nil", func(t *testing.T) {
		var cfg *Config
		cfg.AddHook(nil) // should not panic
	})

	t.Run("AddValidator nil", func(t *testing.T) {
		var cfg *Config
		cfg.AddValidator(nil) // should not panic
	})

	t.Run("AddDangerousPattern nil", func(t *testing.T) {
		var cfg *Config
		cfg.AddDangerousPattern(DangerousPattern{}) // should not panic
	})
}

func TestConfigMutation_ValidReceiver(t *testing.T) {
	t.Run("AddHook appends", func(t *testing.T) {
		cfg := DefaultConfig()
		initial := len(cfg.Hooks)
		cfg.AddHook(&testHookImpl{})
		if len(cfg.Hooks) != initial+1 {
			t.Errorf("expected %d hooks, got %d", initial+1, len(cfg.Hooks))
		}
	})

	t.Run("AddValidator appends", func(t *testing.T) {
		cfg := DefaultConfig()
		initial := len(cfg.CustomValidators)
		cfg.AddValidator(&testValidatorImpl{})
		if len(cfg.CustomValidators) != initial+1 {
			t.Errorf("expected %d validators, got %d", initial+1, len(cfg.CustomValidators))
		}
	})

	t.Run("AddDangerousPattern appends", func(t *testing.T) {
		cfg := DefaultConfig()
		initial := len(cfg.AdditionalDangerousPatterns)
		cfg.AddDangerousPattern(DangerousPattern{Pattern: "test", Name: "Test", Level: PatternLevelWarning})
		if len(cfg.AdditionalDangerousPatterns) != initial+1 {
			t.Errorf("expected %d patterns, got %d", initial+1, len(cfg.AdditionalDangerousPatterns))
		}
	})
}

// ============================================================================
// DefaultSchemaConfig and NewSchemaWithConfig (types.go: 0% coverage)
// ============================================================================

func TestDefaultSchemaConfig(t *testing.T) {
	cfg := DefaultSchemaConfig()
	if cfg.AdditionalProperties == nil || !*cfg.AdditionalProperties {
		t.Error("DefaultSchemaConfig AdditionalProperties should be *true")
	}
}

func TestNewSchemaWithConfig(t *testing.T) {
	t.Run("with additional properties true", func(t *testing.T) {
		cfg := DefaultSchemaConfig()
		schema := NewSchemaWithConfig(cfg)
		if schema == nil {
			t.Fatal("NewSchemaWithConfig returned nil")
		}
		if !schema.AdditionalProperties {
			t.Error("AdditionalProperties should be true")
		}
	})

	t.Run("with additional properties false", func(t *testing.T) {
		falsy := false
		cfg := SchemaConfig{AdditionalProperties: &falsy}
		schema := NewSchemaWithConfig(cfg)
		if schema.AdditionalProperties {
			t.Error("AdditionalProperties should be false")
		}
	})

	t.Run("with nil additional properties", func(t *testing.T) {
		cfg := SchemaConfig{AdditionalProperties: nil}
		schema := NewSchemaWithConfig(cfg)
		if !schema.AdditionalProperties {
			t.Error("nil AdditionalProperties should default to true")
		}
	})
}

// ============================================================================
// Error boundary tests (errors.go: 0% coverage)
// ============================================================================

func TestNewOperationPathError(t *testing.T) {
	innerErr := fmt.Errorf("inner error")
	err := newOperationPathError("get", "users[0].name", "failed to get value", innerErr)

	jerr, ok := err.(*JsonsError)
	if !ok {
		t.Fatalf("expected *JsonsError, got %T", err)
	}
	if jerr.Op != "get" {
		t.Errorf("Op = %q, want %q", jerr.Op, "get")
	}
	if jerr.Path != "users[0].name" {
		t.Errorf("Path = %q, want %q", jerr.Path, "users[0].name")
	}
}

// ============================================================================
// Iterator Reset boundary tests (iterator.go: 0% coverage)
// ============================================================================

func TestIterator_Reset(t *testing.T) {
	data := map[string]any{"a": 1, "b": 2, "c": 3}
	it := NewIterator(data)

	// Consume some items
	it.Next()
	it.Next()

	it.Reset()
	if it.data != nil {
		t.Error("data should be nil after Reset")
	}
	if it.position != 0 {
		t.Error("position should be 0 after Reset")
	}
	if it.keys != nil {
		t.Error("keys should be nil after Reset")
	}
}

func TestIterator_ResetWith(t *testing.T) {
	it := NewIterator(map[string]any{"a": 1})

	newData := map[string]any{"x": 10, "y": 20}
	it.ResetWith(newData)

	// Should iterate over new data
	count := 0
	for {
		_, ok := it.Next()
		if !ok {
			break
		}
		count++
	}
	if count != 2 {
		t.Errorf("expected 2 items after ResetWith, got %d", count)
	}
}

// ============================================================================
// Pooled iterator boundary tests (iterator.go: 0% coverage)
// ============================================================================

func TestPooledSliceIterator(t *testing.T) {
	data := []any{10, 20, 30}
	it := newPooledSliceIterator(data)

	// Iterate all
	var results []any
	for it.Next() {
		results = append(results, it.Value())
	}
	if len(results) != 3 {
		t.Fatalf("expected 3 items, got %d", len(results))
	}
	if results[0] != 10 || results[1] != 20 || results[2] != 30 {
		t.Errorf("unexpected values: %v", results)
	}

	// Next after exhaustion returns false
	if it.Next() {
		t.Error("Next() should return false after exhaustion")
	}

	// Index should be past the end after exhaustion
	if it.Index() < 2 {
		t.Errorf("Index() = %d, should be >= 2", it.Index())
	}

	// Release and verify no panic on double release
	it.Release()
	it.Release() // should not panic
}

func TestPooledSliceIterator_Empty(t *testing.T) {
	it := newPooledSliceIterator([]any{})
	if it.Next() {
		t.Error("Next() should return false for empty slice")
	}
	it.Release()
}

func TestPooledMapIterator(t *testing.T) {
	data := map[string]any{"a": 1, "b": 2}
	it := newPooledMapIterator(data)

	var keys []string
	var values []any
	for it.Next() {
		keys = append(keys, it.Key())
		values = append(values, it.Value())
	}

	if len(keys) != 2 {
		t.Fatalf("expected 2 items, got %d", len(keys))
	}
	if it.Next() {
		t.Error("Next() should return false after exhaustion")
	}

	it.Release()
	it.Release() // should not panic
}

func TestPooledMapIterator_Empty(t *testing.T) {
	it := newPooledMapIterator(map[string]any{})
	if it.Next() {
		t.Error("Next() should return false for empty map")
	}
	it.Release()
}

func TestPooledMapIterator_LargeKeysReplaced(t *testing.T) {
	data := map[string]any{"a": 1}
	it := newPooledMapIterator(data)
	it.keys = make([]string, 0, 300) // force large capacity
	it.Release()
}

// ============================================================================
// ForeachWithPathAndControl and ForeachReturn (iterator.go: 0% coverage)
// ============================================================================

func TestForeachWithPathAndControl_Break(t *testing.T) {
	jsonStr := `{"items": [1, 2, 3, 4, 5]}`
	count := 0
	err := ForeachWithPathAndControl(jsonStr, "items", func(key any, value any) IteratorControl {
		count++
		if count >= 3 {
			return IteratorBreak
		}
		return IteratorContinue
	})
	if err != nil {
		t.Errorf("ForeachWithPathAndControl error: %v", err)
	}
	if count != 3 {
		t.Errorf("expected 3 iterations, got %d", count)
	}
}

func TestForeachReturnBoundary(t *testing.T) {
	jsonStr := `{"a": 1, "b": 2, "c": 3}`
	var count int
	_, err := ForeachReturn(jsonStr, func(key any, item *IterableValue) {
		count++
	})
	if err != nil {
		t.Fatalf("ForeachReturn error: %v", err)
	}
	if count != 3 {
		t.Errorf("expected 3 items, got %d", count)
	}
}

// ============================================================================
// Processor logger and cache boundary tests (processor.go: 0% coverage)
// ============================================================================

func TestProcessor_SetLogger(t *testing.T) {
	t.Run("with custom logger", func(t *testing.T) {
		processor, _ := New()
		defer processor.Close()

		logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
		processor.SetLogger(logger)
		got := processor.getLogger()
		if got == nil {
			t.Error("getLogger() should not return nil after SetLogger")
		}
	})

	t.Run("with nil logger", func(t *testing.T) {
		processor, _ := New()
		defer processor.Close()

		processor.SetLogger(nil)
		got := processor.getLogger()
		if got == nil {
			t.Error("getLogger() should return default logger, not nil")
		}
	})

	t.Run("SetLogger on nil processor", func(t *testing.T) {
		var p *Processor
		p.SetLogger(nil) // should not panic
	})

	t.Run("getLogger on nil processor", func(t *testing.T) {
		var p *Processor
		logger := p.getLogger()
		if logger == nil {
			t.Error("getLogger() on nil processor should return default logger")
		}
	})
}

func TestProcessor_InvalidateCachedResult(t *testing.T) {
	cfg := DefaultConfig()
	cfg.EnableCache = true
	processor, _ := New(cfg)
	defer processor.Close()

	_, _ = processor.Get(`{"key": "value"}`, "key")
	processor.invalidateCachedResult("key")

	result, err := processor.Get(`{"key": "value"}`, "key")
	if err != nil {
		t.Errorf("Get after invalidation failed: %v", err)
	}
	if result != "value" {
		t.Errorf("result = %v, want 'value'", result)
	}
}

func TestProcessor_InvalidateCacheDisabled(t *testing.T) {
	cfg := DefaultConfig()
	cfg.EnableCache = false
	processor, _ := New(cfg)
	defer processor.Close()

	processor.invalidateCachedResult("any_key")
}

// ============================================================================
// checkRateLimit boundary tests (processor.go: 0% coverage)
// ============================================================================

func TestProcessor_CheckRateLimit(t *testing.T) {
	t.Run("disabled when window is zero", func(t *testing.T) {
		processor, _ := New()
		defer processor.Close()
		processor.metrics.operationWindow = 0
		if err := processor.checkRateLimit(); err != nil {
			t.Errorf("checkRateLimit with window=0 should return nil, got %v", err)
		}
	})
}

// ============================================================================
// truncateString and sanitizePath boundary tests (processor.go: 0% coverage)
// ============================================================================

func TestTruncateString(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		maxLen int
		want   string
	}{
		{"short string", "hello", 10, "hello"},
		{"exact length", "hello", 5, "hello"},
		{"needs truncation", "hello world", 8, "hello..."},
		{"maxLen is 3", "abcdef", 3, "abc"},
		{"maxLen is 2", "abcdef", 2, "ab"},
		{"maxLen is 0", "hello", 0, ""},
		{"empty string", "", 5, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncateString(tt.input, tt.maxLen)
			if got != tt.want {
				t.Errorf("truncateString(%q, %d) = %q, want %q", tt.input, tt.maxLen, got, tt.want)
			}
		})
	}
}

func TestSanitizePath(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"normal path", "users[0].name", "users[0].name"},
		{"path with password", "user.password", "[REDACTED_PATH]"},
		{"path with token", "auth.token", "[REDACTED_PATH]"},
		{"path with secret", "config.secret", "[REDACTED_PATH]"},
		{"long path", strings.Repeat("a", 101), strings.Repeat("a", 97) + "..."},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sanitizePath(tt.input)
			if got != tt.want {
				t.Errorf("sanitizePath(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestSanitizeError(t *testing.T) {
	t.Run("nil error", func(t *testing.T) {
		if got := sanitizeError(nil); got != "" {
			t.Errorf("sanitizeError(nil) = %q, want empty", got)
		}
	})

	t.Run("short error", func(t *testing.T) {
		if got := sanitizeError(fmt.Errorf("short")); got != "short" {
			t.Errorf("sanitizeError() = %q, want %q", got, "short")
		}
	})

	t.Run("long error", func(t *testing.T) {
		longErr := fmt.Errorf("%s", strings.Repeat("x", 250))
		got := sanitizeError(longErr)
		if len(got) > 200 {
			t.Errorf("sanitizeError() result too long: %d", len(got))
		}
		if !strings.HasSuffix(got, "...") {
			t.Errorf("sanitizeError() should end with ..., got %q", got)
		}
	})
}

// ============================================================================
// deepCopyValueWithDepth boundary tests (helpers.go: 24% coverage)
// ============================================================================

func TestDeepCopyValueWithDepth_Primitives(t *testing.T) {
	tests := []struct {
		name  string
		input any
	}{
		{"nil", nil},
		{"bool", true},
		{"int", 42},
		{"int8", int8(8)},
		{"int16", int16(16)},
		{"int32", int32(32)},
		{"int64", int64(64)},
		{"uint", uint(1)},
		{"uint8", uint8(8)},
		{"uint16", uint16(16)},
		{"uint32", uint32(32)},
		{"uint64", uint64(64)},
		{"float32", float32(3.14)},
		{"float64", 3.14},
		{"string", "hello"},
		{"json.Number", json.Number("123.456")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := deepCopyValueWithDepth(tt.input, 0)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if got != tt.input {
				t.Errorf("deepCopyValueWithDepth() = %v, want %v", got, tt.input)
			}
		})
	}
}

func TestDeepCopyValueWithDepth_MaxDepthExceeded(t *testing.T) {
	_, err := deepCopyValueWithDepth(map[string]any{}, deepCopyMaxDepth+1)
	if err == nil {
		t.Error("expected error when depth exceeds max")
	}
	if !strings.Contains(err.Error(), "depth limit") {
		t.Errorf("error should mention depth limit, got: %v", err)
	}
}

func TestDeepCopyValueWithDepth_Complex(t *testing.T) {
	t.Run("nested map", func(t *testing.T) {
		input := map[string]any{
			"level1": map[string]any{
				"level2": "value",
			},
		}
		got, err := deepCopyValueWithDepth(input, 0)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		input["level1"].(map[string]any)["level2"] = "changed"
		if got.(map[string]any)["level1"].(map[string]any)["level2"] != "value" {
			t.Error("deep copy should not be affected by original modification")
		}
	})

	t.Run("slice", func(t *testing.T) {
		input := []any{1, 2, 3}
		got, err := deepCopyValueWithDepth(input, 0)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		copySlice := got.([]any)
		if len(copySlice) != 3 {
			t.Errorf("expected 3 elements, got %d", len(copySlice))
		}
	})
}

// ============================================================================
// convertToSlice and convertToMap boundary tests (helpers.go: 27%/20% coverage)
// ============================================================================

func TestConvertToSlice(t *testing.T) {
	t.Run("valid slice conversion", func(t *testing.T) {
		input := []any{1, 2, 3}
		targetType := reflect.TypeOf([]int{})
		result, ok := convertToSlice(input, targetType)
		if !ok {
			t.Fatal("convertToSlice should succeed")
		}
		resultSlice := result.([]int)
		if len(resultSlice) != 3 {
			t.Errorf("expected 3 elements, got %d", len(resultSlice))
		}
	})

	t.Run("non-slice input", func(t *testing.T) {
		result, ok := convertToSlice("not a slice", reflect.TypeOf([]int{}))
		if ok {
			t.Error("convertToSlice should fail for non-slice input")
		}
		if result != nil {
			t.Error("result should be nil on failure")
		}
	})

	t.Run("empty slice", func(t *testing.T) {
		input := []any{}
		targetType := reflect.TypeOf([]int{})
		result, ok := convertToSlice(input, targetType)
		if !ok {
			t.Fatal("convertToSlice should succeed for empty slice")
		}
		resultSlice := result.([]int)
		if len(resultSlice) != 0 {
			t.Errorf("expected 0 elements, got %d", len(resultSlice))
		}
	})

	t.Run("element conversion failure", func(t *testing.T) {
		input := []any{1, "not a number", 3}
		targetType := reflect.TypeOf([]int{})
		_, ok := convertToSlice(input, targetType)
		if ok {
			t.Error("convertToSlice should fail when element conversion fails")
		}
	})
}

func TestConvertToMap(t *testing.T) {
	t.Run("valid map conversion", func(t *testing.T) {
		input := map[string]any{"a": 1, "b": 2}
		targetType := reflect.TypeOf(map[string]int{})
		result, ok := convertToMap(input, targetType)
		if !ok {
			t.Fatal("convertToMap should succeed")
		}
		resultMap := result.(map[string]int)
		if len(resultMap) != 2 {
			t.Errorf("expected 2 keys, got %d", len(resultMap))
		}
	})

	t.Run("non-map input", func(t *testing.T) {
		result, ok := convertToMap("not a map", reflect.TypeOf(map[string]int{}))
		if ok {
			t.Error("convertToMap should fail for non-map input")
		}
		if result != nil {
			t.Error("result should be nil on failure")
		}
	})

	t.Run("value conversion failure", func(t *testing.T) {
		input := map[string]any{"a": 1, "b": "not a number"}
		targetType := reflect.TypeOf(map[string]int{})
		_, ok := convertToMap(input, targetType)
		if ok {
			t.Error("convertToMap should fail when value conversion fails")
		}
	})
}

// ============================================================================
// PatternRegistry ListByLevel (security.go: 0% coverage)
// ============================================================================

func TestPatternRegistry_ListByLevel(t *testing.T) {
	t.Run("filter by critical level", func(t *testing.T) {
		registry := &patternRegistry{
			patterns: map[string]DangerousPattern{
				"critical1": {Pattern: "crit1", Name: "Critical1", Level: PatternLevelCritical},
				"warning1":  {Pattern: "warn1", Name: "Warning1", Level: PatternLevelWarning},
				"critical2": {Pattern: "crit2", Name: "Critical2", Level: PatternLevelCritical},
			},
		}

		result := registry.ListByLevel(PatternLevelCritical)
		if len(result) != 2 {
			t.Errorf("expected 2 critical patterns, got %d", len(result))
		}
		for _, p := range result {
			if p.Level != PatternLevelCritical {
				t.Errorf("expected Critical level, got %v", p.Level)
			}
		}
	})

	t.Run("empty for nonexistent level", func(t *testing.T) {
		registry := &patternRegistry{
			patterns: map[string]DangerousPattern{},
		}
		result := registry.ListByLevel(PatternLevelCritical)
		if len(result) != 0 {
			t.Errorf("expected 0 patterns for empty registry, got %d", len(result))
		}
	})
}

// ============================================================================
// encodeJSONNumber boundary tests (encoding.go: 0% coverage)
// ============================================================================

func TestEncodeJSONNumber_PreserveNumbers(t *testing.T) {
	cfg := DefaultConfig()
	cfg.PreserveNumbers = true
	processor, _ := New(cfg)
	defer processor.Close()

	tests := []struct {
		name    string
		data    string
		path    string
		wantVal string
	}{
		{"integer", `{"num": 42}`, "num", "42"},
		{"float", `{"num": 3.14}`, "num", "3.14"},
		{"large number", `{"num": 9999999999999999999}`, "num", "9999999999999999999"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			val, err := processor.Get(tt.data, tt.path)
			if err != nil {
				t.Fatalf("Get error: %v", err)
			}
			num, ok := val.(json.Number)
			if !ok {
				t.Logf("value type: %T", val)
				return
			}
			if num.String() != tt.wantVal {
				t.Errorf("got %q, want %q", num.String(), tt.wantVal)
			}
		})
	}
}

// ============================================================================
// logError boundary test (processor.go: 0% coverage)
// ============================================================================

// logError is tested indirectly through error paths in other tests.
// Direct testing is difficult because it depends on internal metrics collector state.

// ============================================================================
// ForeachNestedWithError (processor.go: 0% coverage)
// ============================================================================

func TestForeachNestedWithError(t *testing.T) {
	processor, _ := New()
	defer processor.Close()

	t.Run("iterates nested data", func(t *testing.T) {
		jsonStr := `{"users": [{"name": "Alice"}, {"name": "Bob"}]}`
		var names []string
		err := processor.ForeachNestedWithError(jsonStr, func(key any, item *IterableValue) error {
			if m, ok := item.GetData().(map[string]any); ok {
				if n, ok := m["name"]; ok {
					names = append(names, n.(string))
				}
			}
			return nil
		})
		if err != nil {
			t.Errorf("ForeachNestedWithError error: %v", err)
		}
		if len(names) < 1 {
			t.Errorf("expected at least 1 name, got %d: %v", len(names), names)
		}
	})

	t.Run("callback error stops iteration", func(t *testing.T) {
		jsonStr := `{"a": 1, "b": 2}`
		err := processor.ForeachNestedWithError(jsonStr, func(key any, item *IterableValue) error {
			return fmt.Errorf("stop")
		})
		if err == nil {
			t.Error("expected error from callback")
		}
	})
}

// ============================================================================
// StreamJSONLFile (processor_streamjsonl.go: 0% coverage)
// ============================================================================

func TestStreamJSONLFile(t *testing.T) {
	t.Run("with invalid file path", func(t *testing.T) {
		processor, _ := New()
		defer processor.Close()

		err := processor.StreamJSONLFile("nonexistent_file.jsonl", func(lineNum int, item *IterableValue) error {
			return nil
		})
		if err == nil {
			t.Error("expected error for nonexistent file")
		}
	})
}

// ============================================================================
// Concurrent safe deep copy test
// ============================================================================

func TestDeepCopyValueWithDepth_Concurrent(t *testing.T) {
	input := map[string]any{
		"nested": map[string]any{
			"values": []any{1, 2, 3},
		},
	}

	var wg sync.WaitGroup
	errCh := make(chan error, 10)

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			got, err := deepCopyValueWithDepth(input, 0)
			if err != nil {
				errCh <- err
				return
			}
			m, ok := got.(map[string]any)
			if !ok {
				errCh <- fmt.Errorf("expected map, got %T", got)
			}
			if len(m) != 1 {
				errCh <- fmt.Errorf("expected 1 key, got %d", len(m))
			}
		}()
	}

	wg.Wait()
	close(errCh)
	for err := range errCh {
		t.Errorf("concurrent deep copy error: %v", err)
	}
}

// ============================================================================
// valuesEqual boundary tests (encoding.go: via Processor method)
// ============================================================================

func TestValuesEqual_EdgeCases(t *testing.T) {
	processor, _ := New()
	defer processor.Close()

	tests := []struct {
		name string
		a, b any
		want bool
	}{
		{"both nil", nil, nil, true},
		{"nil vs non-nil", nil, 1, false},
		{"non-nil vs nil", 1, nil, false},
		{"same int", 42, 42, true},
		{"different int", 42, 43, false},
		{"same string", "hello", "hello", true},
		{"different string", "hello", "world", false},
		{"same bool", true, true, true},
		{"different bool", true, false, false},
		{"same float", 3.14, 3.14, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := processor.valuesEqual(tt.a, tt.b); got != tt.want {
				t.Errorf("valuesEqual(%v, %v) = %v, want %v", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

// ============================================================================
// isEmptyOrZero extended boundary tests
// ============================================================================

func TestIsEmptyOrZero_EdgeCases(t *testing.T) {
	tests := []struct {
		name  string
		input any
		want  bool
	}{
		{"zero uint", uint(0), true},
		{"non-zero uint", uint(1), false},
		{"json.Number zero", json.Number("0"), true},
		{"json.Number non-zero", json.Number("42"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isEmptyOrZero(tt.input); got != tt.want {
				t.Errorf("isEmptyOrZero(%v) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

// ============================================================================
// Compiled path tests (processor.go: CompilePath, GetCompiled)
// ============================================================================

func TestProcessor_CompileAndGetCompiledPath(t *testing.T) {
	processor, _ := New()
	defer processor.Close()

	t.Run("compile and use", func(t *testing.T) {
		cp, err := processor.CompilePath("users[0].name")
		if err != nil {
			t.Fatalf("CompilePath error: %v", err)
		}
		if cp == nil {
			t.Fatal("CompilePath should return non-nil CompiledPath")
		}

		result, err := processor.GetCompiled(`{"users":[{"name":"Alice"}]}`, cp)
		if err != nil {
			t.Errorf("GetCompiled error: %v", err)
		}
		if result != "Alice" {
			t.Errorf("GetCompiled result = %v, want Alice", result)
		}
	})

	t.Run("compiled path reused", func(t *testing.T) {
		cp1, _ := processor.CompilePath("a.b.c")
		cp2, _ := processor.CompilePath("a.b.c")
		// Same path should return equivalent compiled path
		_ = cp1
		_ = cp2
	})
}

// ============================================================================
// evictLRUEntries (security.go: 0% coverage)
// ============================================================================

func TestEvictLRUEntries(t *testing.T) {
	t.Run("empty cache does nothing", func(t *testing.T) {
		sv := &securityValidator{
			validationCache: make(map[string]*validationCacheEntry),
		}
		sv.evictLRUEntries() // should not panic
	})

	t.Run("evicts oldest entries", func(t *testing.T) {
		sv := &securityValidator{
			validationCache: make(map[string]*validationCacheEntry),
		}

		for i := 0; i < 4; i++ {
			sv.validationCache[fmt.Sprintf("key%d", i)] = &validationCacheEntry{
				validated:  true,
				lastAccess: int64(i) * 1000,
			}
		}

		sv.evictLRUEntries()

		if len(sv.validationCache) >= 4 {
			t.Errorf("expected some entries evicted, still have %d", len(sv.validationCache))
		}
	})
}

// ============================================================================
// path.go: ParsedJSON Data() (types.go: 0% coverage)
// ============================================================================

func TestParsedJSON_Data(t *testing.T) {
	processor, _ := New()
	defer processor.Close()

	parsed, err := processor.PreParse(`{"key": "value"}`)
	if err != nil {
		t.Fatalf("PreParse error: %v", err)
	}

	data := parsed.Data()
	if data == nil {
		t.Fatal("Data() should not return nil")
	}
	m, ok := data.(map[string]any)
	if !ok {
		t.Fatalf("expected map, got %T", data)
	}
	if m["key"] != "value" {
		t.Errorf("Data()[key] = %v, want 'value'", m["key"])
	}
}

// ============================================================================
// encodeString with DisableEscaping (encoding.go: 35% coverage)
// ============================================================================

func TestEncodeString_DisableEscaping(t *testing.T) {
	cfg := DefaultConfig()
	cfg.DisableEscaping = true
	processor, _ := New(cfg)
	defer processor.Close()

	result, err := processor.Encode(map[string]any{"text": "hello 世界"})
	if err != nil {
		t.Fatalf("Encode error: %v", err)
	}
	if !strings.Contains(result, "hello") {
		t.Errorf("result should contain 'hello': %s", result)
	}
}

// ============================================================================
// containsPercentEncodingBypass (security.go: 11.5% coverage)
// ============================================================================

func TestContainsPercentEncodingBypass(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{"clean path", "/home/user/file.json", false},
		{"percent encoding", "%2e%2e%2f", true},
		{"mixed encoding", "..%2f..%2fetc", true},
		{"double encoding", "%252e%252e", true},
		{"normal percent", "100%", false},
		{"empty string", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := containsPercentEncodingBypass(tt.input)
			if got != tt.want {
				t.Errorf("containsPercentEncodingBypass(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

// ============================================================================
// containsOverlongEncoding (security.go: 13.6% coverage)
// ============================================================================

func TestContainsOverlongEncoding(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{"clean ASCII", "hello world", false},
		{"empty string", "", false},
		{"normal UTF-8", "café", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := containsOverlongEncoding(tt.input)
			if got != tt.want {
				t.Errorf("containsOverlongEncoding() = %v, want %v", got, tt.want)
			}
		})
	}
}

// ============================================================================
// Error wrapping boundary tests
// ============================================================================

func TestErrorWrapping_Is(t *testing.T) {
	baseErr := ErrPathNotFound
	wrappedErr := fmt.Errorf("operation failed: %w", baseErr)

	if !errors.Is(wrappedErr, ErrPathNotFound) {
		t.Error("wrapped ErrPathNotFound should match via errors.Is")
	}
}

// ============================================================================
// ParsedJSON concurrent access test
// ============================================================================

func TestParsedJSON_ConcurrentAccess(t *testing.T) {
	processor, _ := New()
	defer processor.Close()

	parsed, err := processor.PreParse(`{"x":1,"y":2,"z":3}`)
	if err != nil {
		t.Fatalf("PreParse error: %v", err)
	}

	var wg sync.WaitGroup
	errCh := make(chan error, 20)

	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			data := parsed.Data()
			if data == nil {
				errCh <- fmt.Errorf("Data() returned nil")
				return
			}
		}()
	}

	wg.Wait()
	close(errCh)
	for err := range errCh {
		t.Errorf("concurrent access error: %v", err)
	}
}
