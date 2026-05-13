package json

import (
	"reflect"
	"testing"
	"time"
)

// ============================================================================
// HASH CUSTOM ESCAPES TESTS
// ============================================================================

// TestHashCustomEscapes tests the hashCustomEscapes function
func TestHashCustomEscapes(t *testing.T) {
	tests := []struct {
		name string
		m    map[rune]string
	}{
		{"nil map", nil},
		{"empty map", map[rune]string{}},
		{"single entry", map[rune]string{'<': "\\u003c"}},
		{"multiple entries", map[rune]string{'<': "\\u003c", '>': "\\u003e", '&': "\\u0026"}},
		{"unicode key", map[rune]string{'\u2028': "\\u2028"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test that hashing is deterministic
			h1 := hashCustomEscapes(0, tt.m)
			h2 := hashCustomEscapes(0, tt.m)
			if h1 != h2 {
				t.Errorf("hashCustomEscapes is not deterministic: %d != %d", h1, h2)
			}

			// Test that different starting hashes produce different results
			h3 := hashCustomEscapes(1000, tt.m)
			if len(tt.m) > 0 && h1 == h3 {
				t.Errorf("Different starting hashes should produce different results")
			}
		})
	}
}

// TestHashCustomEscapes_DifferentMaps tests that different maps produce different hashes
func TestHashCustomEscapes_DifferentMaps(t *testing.T) {
	maps := []map[rune]string{
		nil,
		{},
		{'<': "\\u003c"},
		{'>': "\\u003e"},
		{'<': "\\u003c", '>': "\\u003e"},
		{'&': "\\u0026"},
	}

	hashes := make(map[uint64]int)
	for i, m := range maps {
		h := hashCustomEscapes(0, m)
		if other, exists := hashes[h]; exists {
			// Some maps might have the same hash (collision), but it's unlikely for these test cases
			t.Logf("Hash collision between map[%d] and map[%d]: %d", i, other, h)
		}
		hashes[h] = i
	}
}

// ============================================================================
// CONFIG FIELDS EQUAL TESTS
// ============================================================================

// TestConfigFieldsEqual tests the configFieldsEqual function with various configs
func TestConfigFieldsEqual(t *testing.T) {
	tests := []struct {
		name     string
		a        Config
		b        Config
		expected bool
	}{
		{
			name:     "both default",
			a:        DefaultConfig(),
			b:        DefaultConfig(),
			expected: true,
		},
		{
			name:     "different MaxJSONSize",
			a:        Config{MaxJSONSize: 1000},
			b:        Config{MaxJSONSize: 2000},
			expected: false,
		},
		{
			name:     "different EnableCache",
			a:        Config{EnableCache: true},
			b:        Config{EnableCache: false},
			expected: false,
		},
		{
			name:     "different CustomEscapes",
			a:        Config{CustomEscapes: map[rune]string{'<': "\\u003c"}},
			b:        Config{CustomEscapes: nil},
			expected: false,
		},
		{
			name:     "equal CustomEscapes",
			a:        Config{CustomEscapes: map[rune]string{'<': "\\u003c"}},
			b:        Config{CustomEscapes: map[rune]string{'<': "\\u003c"}},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := configFieldsEqual(tt.a, tt.b)
			if result != tt.expected {
				t.Errorf("configFieldsEqual() = %v, want %v", result, tt.expected)
			}
		})
	}
}

// ============================================================================
// IS DEFAULT CONFIG TESTS
// ============================================================================

// TestIsDefaultConfig tests the isDefaultConfig function
func TestIsDefaultConfig(t *testing.T) {
	tests := []struct {
		name     string
		config   Config
		expected bool
	}{
		{"default config", DefaultConfig(), true},
		{"modified MaxJSONSize", func() Config { c := DefaultConfig(); c.MaxJSONSize = 999; return c }(), false},
		{"modified EnableCache", func() Config { c := DefaultConfig(); c.EnableCache = !c.EnableCache; return c }(), false},
		{"with CustomEscapes", func() Config { c := DefaultConfig(); c.CustomEscapes = map[rune]string{'<': "\\u003c"}; return c }(), false},
		{"zero config", Config{}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isDefaultConfig(tt.config)
			if result != tt.expected {
				t.Errorf("isDefaultConfig() = %v, want %v", result, tt.expected)
			}
		})
	}
}

// ============================================================================
// HASH CONFIG TESTS
// ============================================================================

// TestHashConfig tests the hashConfig function
func TestHashConfig(t *testing.T) {
	t.Run("deterministic", func(t *testing.T) {
		c := DefaultConfig()
		h1 := hashConfig(c)
		h2 := hashConfig(c)
		if h1 != h2 {
			t.Errorf("hashConfig is not deterministic: %d != %d", h1, h2)
		}
	})

	t.Run("different configs different hashes", func(t *testing.T) {
		c1 := DefaultConfig()
		c2 := DefaultConfig()
		c2.MaxJSONSize = 999

		h1 := hashConfig(c1)
		h2 := hashConfig(c2)

		if h1 == h2 {
			t.Errorf("Different configs should have different hashes")
		}
	})

	t.Run("with custom escapes", func(t *testing.T) {
		c := DefaultConfig()
		c.CustomEscapes = map[rune]string{'<': "\\u003c"}

		h := hashConfig(c)
		if h == 0 {
			t.Error("hashConfig with CustomEscapes should not be 0")
		}
	})
}

// ============================================================================
// PROCESS RECURSIVELY WITH OPTIONS TESTS
// ============================================================================

// TestProcessRecursivelyWithOptions_Comprehensive tests more edge cases
func TestProcessRecursivelyWithOptions_Comprehensive(t *testing.T) {
	processor, _ := New()
	defer processor.Close()
	rp := newRecursiveProcessor(processor)

	t.Run("Get with createPaths false", func(t *testing.T) {
		data := map[string]any{"existing": "value"}
		result, err := rp.ProcessRecursivelyWithOptions(data, "existing", opGet, nil, false)
		if err != nil {
			t.Errorf("Get existing path failed: %v", err)
		}
		if result != "value" {
			t.Errorf("Get = %v, want 'value'", result)
		}
	})

	t.Run("Get non-existing without createPaths", func(t *testing.T) {
		data := map[string]any{}
		_, err := rp.ProcessRecursivelyWithOptions(data, "nonexistent", opGet, nil, false)
		if err == nil {
			t.Error("Expected error for non-existing path")
		}
	})

	t.Run("Set without createPaths on new path", func(t *testing.T) {
		data := map[string]any{}
		_, err := rp.ProcessRecursivelyWithOptions(data, "new.path", opSet, "value", false)
		if err == nil {
			t.Error("Expected error for setting non-existing path without createPaths")
		}
	})

	t.Run("Delete root should fail", func(t *testing.T) {
		data := map[string]any{"key": "value"}
		_, err := rp.ProcessRecursivelyWithOptions(data, "", opDelete, nil, false)
		if err == nil {
			t.Error("Expected error for deleting root")
		}
	})

	t.Run("Set root should fail", func(t *testing.T) {
		data := map[string]any{"key": "value"}
		_, err := rp.ProcessRecursivelyWithOptions(data, "", opSet, "newroot", false)
		if err == nil {
			t.Error("Expected error for setting root")
		}
	})

	t.Run("Nested array operations", func(t *testing.T) {
		data := map[string]any{
			"items": []any{
				map[string]any{"id": 1},
				map[string]any{"id": 2},
			},
		}

		result, err := rp.ProcessRecursivelyWithOptions(data, "items[0].id", opGet, nil, false)
		if err != nil {
			t.Errorf("Get nested array failed: %v", err)
		}
		// Result can be int or float64 depending on how it was stored
		if result != 1 && result != float64(1) {
			t.Errorf("Get = %v (%T), want 1", result, result)
		}
	})
}

// ============================================================================
// BENCHMARK TESTS
// ============================================================================

func BenchmarkHashCustomEscapes(b *testing.B) {
	m := map[rune]string{'<': "\\u003c", '>': "\\u003e", '&': "\\u0026"}

	for i := 0; i < b.N; i++ {
		_ = hashCustomEscapes(0, m)
	}
}

func BenchmarkHashConfig(b *testing.B) {
	c := DefaultConfig()

	for i := 0; i < b.N; i++ {
		_ = hashConfig(c)
	}
}

// ============================================================================
// CONFIG FIELD SYNC TESTS
// Ensures configFieldsEqual and hashConfigFields handle the same fields
// ============================================================================

// TestConfigFieldSync verifies that configFieldsEqual and hashConfigFields
// are kept in sync. This test uses reflection to check all Config fields
// and ensures modifications are properly detected by both functions.
//
// MAINTENANCE: When adding new fields to Config, ensure both functions
// handle the new field. This test will help catch omissions.
func TestConfigFieldSync(t *testing.T) {
	// Test that modifying each field individually changes the hash
	// and breaks equality with default config
	defaultCfg := DefaultConfig()

	// List of fields to test (all exported fields of Config)
	// When adding new fields to Config, add them here
	fieldsToTest := []struct {
		name   string
		modify func(*Config)
	}{
		// Cache settings
		{"MaxCacheSize", func(c *Config) { c.MaxCacheSize = 999 }},
		{"CacheTTL", func(c *Config) { c.CacheTTL = 99 * time.Second }},
		{"EnableCache", func(c *Config) { c.EnableCache = !c.EnableCache }},
		{"CacheResults", func(c *Config) { c.CacheResults = !c.CacheResults }},

		// Size limits
		{"MaxJSONSize", func(c *Config) { c.MaxJSONSize = 999 }},
		{"MaxPathDepth", func(c *Config) { c.MaxPathDepth = 99 }},
		{"MaxBatchSize", func(c *Config) { c.MaxBatchSize = 99 }},

		// Security limits
		{"MaxNestingDepthSecurity", func(c *Config) { c.MaxNestingDepthSecurity = 99 }},
		{"MaxSecurityValidationSize", func(c *Config) { c.MaxSecurityValidationSize = 999 }},
		{"MaxObjectKeys", func(c *Config) { c.MaxObjectKeys = 99 }},
		{"MaxArrayElements", func(c *Config) { c.MaxArrayElements = 99 }},
		{"FullSecurityScan", func(c *Config) { c.FullSecurityScan = !c.FullSecurityScan }},

		// Concurrency
		{"MaxConcurrency", func(c *Config) { c.MaxConcurrency = 99 }},
		{"ParallelThreshold", func(c *Config) { c.ParallelThreshold = 99 }},

		// Processing options
		{"EnableValidation", func(c *Config) { c.EnableValidation = !c.EnableValidation }},
		{"StrictMode", func(c *Config) { c.StrictMode = !c.StrictMode }},
		{"CreatePaths", func(c *Config) { c.CreatePaths = !c.CreatePaths }},
		{"CleanupNulls", func(c *Config) { c.CleanupNulls = !c.CleanupNulls }},
		{"CompactArrays", func(c *Config) { c.CompactArrays = !c.CompactArrays }},
		{"ContinueOnError", func(c *Config) { c.ContinueOnError = !c.ContinueOnError }},

		// Input/Output options
		{"AllowComments", func(c *Config) { c.AllowComments = !c.AllowComments }},
		{"PreserveNumbers", func(c *Config) { c.PreserveNumbers = !c.PreserveNumbers }},
		{"ValidateInput", func(c *Config) { c.ValidateInput = !c.ValidateInput }},
		{"ValidateFilePath", func(c *Config) { c.ValidateFilePath = !c.ValidateFilePath }},
		{"SkipValidation", func(c *Config) { c.SkipValidation = !c.SkipValidation }},

		// Encoding options
		{"Pretty", func(c *Config) { c.Pretty = !c.Pretty }},
		{"Indent", func(c *Config) { c.Indent = "    " }},
		{"Prefix", func(c *Config) { c.Prefix = "  " }},
		{"EscapeHTML", func(c *Config) { c.EscapeHTML = !c.EscapeHTML }},
		{"SortKeys", func(c *Config) { c.SortKeys = !c.SortKeys }},
		{"ValidateUTF8", func(c *Config) { c.ValidateUTF8 = !c.ValidateUTF8 }},
		{"MaxDepth", func(c *Config) { c.MaxDepth = 99 }},
		{"DisallowUnknown", func(c *Config) { c.DisallowUnknown = !c.DisallowUnknown }},
		{"FloatPrecision", func(c *Config) { c.FloatPrecision = 10 }},
		{"FloatTruncate", func(c *Config) { c.FloatTruncate = !c.FloatTruncate }},
		{"DisableEscaping", func(c *Config) { c.DisableEscaping = !c.DisableEscaping }},
		{"EscapeUnicode", func(c *Config) { c.EscapeUnicode = !c.EscapeUnicode }},
		{"EscapeSlash", func(c *Config) { c.EscapeSlash = !c.EscapeSlash }},
		{"EscapeNewlines", func(c *Config) { c.EscapeNewlines = !c.EscapeNewlines }},
		{"EscapeTabs", func(c *Config) { c.EscapeTabs = !c.EscapeTabs }},
		{"IncludeNulls", func(c *Config) { c.IncludeNulls = !c.IncludeNulls }},

		// Observability
		{"EnableMetrics", func(c *Config) { c.EnableMetrics = !c.EnableMetrics }},
		{"EnableHealthCheck", func(c *Config) { c.EnableHealthCheck = !c.EnableHealthCheck }},

		// CustomEscapes
		{"CustomEscapes", func(c *Config) { c.CustomEscapes = map[rune]string{'<': "\\u003c"} }},
	}

	defaultHash := hashConfig(defaultCfg)

	for _, tt := range fieldsToTest {
		t.Run(tt.name, func(t *testing.T) {
			// Create modified config
			modified := DefaultConfig()
			tt.modify(&modified)

			// Test 1: configFieldsEqual should return false
			if configFieldsEqual(defaultCfg, modified) {
				t.Errorf("configFieldsEqual returned true for modified field %s - field may be missing from comparison", tt.name)
			}

			// Test 2: hashConfig should produce different hash
			modifiedHash := hashConfig(modified)
			if defaultHash == modifiedHash {
				t.Errorf("hashConfig produced same hash for modified field %s - field may be missing from hash", tt.name)
			}

			// Test 3: Modified config should not be equal to itself after reset
			resetModified := DefaultConfig()
			tt.modify(&resetModified)
			if !configFieldsEqual(modified, resetModified) {
				t.Errorf("configFieldsEqual returned false for identical configs with field %s", tt.name)
			}
		})
	}
}

// ============================================================================
// MAINTENANCE GUARDRAIL TESTS
// Ensure internal bookkeeping stays in sync with Config struct changes.
// ============================================================================

// TestConfigFieldListCoverage verifies that configFieldList covers all exported
// Config fields. If a new field is added to Config but not to configFieldList,
// config hashing/comparison silently breaks. This test prevents that.
func TestConfigFieldListCoverage(t *testing.T) {
	configType := reflect.TypeOf(Config{})

	// Build set of names in configFieldList
	fieldListNames := make(map[string]bool, len(configFieldList))
	for _, accessor := range configFieldList {
		fieldListNames[accessor.name] = true
	}

	// Check every exported Config field is covered
	for i := 0; i < configType.NumField(); i++ {
		field := configType.Field(i)
		if !field.IsExported() {
			continue
		}
		if !fieldListNames[field.Name] {
			t.Errorf("Config field %q is missing from configFieldList; "+
				"add an entry to configFieldList in api.go to ensure correct "+
				"config comparison and hashing", field.Name)
		}
	}

	// Check for stale entries in configFieldList (names that no longer exist in Config)
	configFields := make(map[string]bool, configType.NumField())
	for i := 0; i < configType.NumField(); i++ {
		configFields[configType.Field(i).Name] = true
	}
	for _, accessor := range configFieldList {
		if !configFields[accessor.name] {
			t.Errorf("configFieldList contains %q but Config has no such field; "+
				"remove the stale entry from configFieldList", accessor.name)
		}
	}
}

// TestReleaseConfigCoversAllReferenceFields verifies that releaseConfig and
// prepareOptions clear all reference-type fields in Config. If a new reference
// field is added but not cleared, pooled Config objects leak data between operations.
func TestReleaseConfigCoversAllReferenceFields(t *testing.T) {
	configType := reflect.TypeOf(Config{})

	// Collect all reference-type fields from Config
	var refFields []string
	for i := 0; i < configType.NumField(); i++ {
		field := configType.Field(i)
		if !field.IsExported() {
			continue
		}
		kind := field.Type.Kind()
		isRef := kind == reflect.Slice || kind == reflect.Map ||
			kind == reflect.Chan || kind == reflect.Func ||
			kind == reflect.Interface || kind == reflect.Ptr
		// context.Context is an interface
		if !isRef && field.Type.String() == "context.Context" {
			isRef = true
		}
		if isRef {
			refFields = append(refFields, field.Name)
		}
	}

	// Known reference-type fields that are cleared in releaseConfig/prepareOptions
	knownRefFields := map[string]bool{
		"CustomEncoder":               true,
		"CustomPathParser":            true,
		"CustomEscapes":               true,
		"CustomTypeEncoders":          true,
		"CustomValidators":            true,
		"AdditionalDangerousPatterns": true,
		"Hooks":                       true,
	}

	for _, field := range refFields {
		if !knownRefFields[field] {
			t.Errorf("Config has reference-type field %q that is NOT cleared in "+
				"releaseConfig/prepareOptions. Add clearing logic to prevent pool "+
				"contamination.", field)
		}
	}
}

// --- Typed getter default-value branches ---

func TestTypedGetters_Defaults(t *testing.T) {
	tests := []struct {
		name string
		fn   func() any
		want any
	}{
		{name: "GetString missing returns default", fn: func() any { return GetString(`{"a":1}`, "missing", "default") }, want: "default"},
		{name: "GetInt type mismatch returns default", fn: func() any { return GetInt(`{"a":"not_int"}`, "a", 42) }, want: 42},
		{name: "GetFloat missing returns default", fn: func() any { return GetFloat(`{}`, "missing", 3.14) }, want: 3.14},
		{name: "GetBool missing returns default", fn: func() any { return GetBool(`{}`, "missing", true) }, want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.fn()
			if got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

// --- getProcessorWithConfig cache paths ---

func TestGetProcessorWithConfig_CacheBehavior(t *testing.T) {
	t.Run("cache hit returns same processor", func(t *testing.T) {
		cfg := DefaultConfig()
		p1, err := getProcessorWithConfig(cfg)
		if err != nil {
			t.Fatalf("getProcessorWithConfig failed: %v", err)
		}
		p2, err := getProcessorWithConfig(cfg)
		if err != nil {
			t.Fatalf("getProcessorWithConfig failed: %v", err)
		}
		if p1 != p2 {
			t.Error("expected same processor from cache for identical config")
		}
	})

	t.Run("different configs return different processors", func(t *testing.T) {
		cfg1 := DefaultConfig()
		cfg2 := DefaultConfig()
		cfg2.EnableCache = false
		p1, err := getProcessorWithConfig(cfg1)
		if err != nil {
			t.Fatalf("getProcessorWithConfig failed: %v", err)
		}
		p2, err := getProcessorWithConfig(cfg2)
		if err != nil {
			t.Fatalf("getProcessorWithConfig failed: %v", err)
		}
		if p1 == p2 {
			t.Error("expected different processors for different configs")
		}
	})
}
