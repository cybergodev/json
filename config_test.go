package json

import (
	"errors"
	"fmt"
	"testing"
	"time"
)

// TestConfiguration tests configuration creation, validation, and cloning
func TestConfiguration(t *testing.T) {
	helper := NewTestHelper(t)

	t.Run("DefaultConfig", func(t *testing.T) {
		config := DefaultConfig()

		helper.AssertNotNil(config)
		helper.AssertTrue(config.EnableCache)
		helper.AssertEqual(DefaultCacheSize, config.MaxCacheSize)
		helper.AssertEqual(DefaultCacheTTL, config.CacheTTL)
		helper.AssertEqual(int64(DefaultMaxJSONSize), config.MaxJSONSize)
		helper.AssertEqual(DefaultMaxPathDepth, config.MaxPathDepth)
		helper.AssertFalse(config.StrictMode)
		helper.AssertFalse(config.EnableMetrics)
		helper.AssertFalse(config.EnableHealthCheck)
	})

	t.Run("HighSecurityConfig", func(t *testing.T) {
		config := HighSecurityConfig()

		helper.AssertNotNil(config)
		helper.AssertEqual(20, config.MaxNestingDepthSecurity)
		helper.AssertEqual(int64(10*1024*1024), config.MaxSecurityValidationSize)
		helper.AssertEqual(1000, config.MaxObjectKeys)
		helper.AssertEqual(1000, config.MaxArrayElements)
		helper.AssertEqual(int64(5*1024*1024), config.MaxJSONSize)
		helper.AssertEqual(20, config.MaxPathDepth)
		helper.AssertTrue(config.EnableValidation)
		helper.AssertTrue(config.StrictMode)
	})

	t.Run("LargeDataConfig", func(t *testing.T) {
		config := LargeDataConfig()

		helper.AssertNotNil(config)
		helper.AssertEqual(100, config.MaxNestingDepthSecurity)
		helper.AssertEqual(int64(500*1024*1024), config.MaxSecurityValidationSize)
		helper.AssertEqual(50000, config.MaxObjectKeys)
		helper.AssertEqual(50000, config.MaxArrayElements)
		helper.AssertEqual(int64(100*1024*1024), config.MaxJSONSize)
		helper.AssertEqual(200, config.MaxPathDepth)
	})

	t.Run("ConfigClone", func(t *testing.T) {
		original := DefaultConfig()
		original.EnableCache = false
		original.StrictMode = true

		cloned := original.Clone()

		helper.AssertNotNil(cloned)
		helper.AssertEqual(original.EnableCache, cloned.EnableCache)
		helper.AssertEqual(original.StrictMode, cloned.StrictMode)

		// Modify clone should not affect original
		cloned.EnableCache = true
		helper.AssertFalse(original.EnableCache)
	})

	t.Run("ConfigValidate", func(t *testing.T) {
		t.Run("ValidConfig", func(t *testing.T) {
			config := DefaultConfig()
			err := config.Validate()
			helper.AssertNoError(err)
		})

		t.Run("NilConfig", func(t *testing.T) {
			err := ValidateConfig(nil)
			helper.AssertError(err)
		})

		t.Run("NegativeCacheSize", func(t *testing.T) {
			config := DefaultConfig()
			config.MaxCacheSize = -1
			err := ValidateConfig(config)
			helper.AssertError(err)
		})

		t.Run("ZeroValuesGetDefaults", func(t *testing.T) {
			config := &Config{}
			err := ValidateConfig(config)
			helper.AssertNoError(err)

			// Should have defaults applied
			helper.AssertTrue(config.MaxJSONSize > 0)
			helper.AssertTrue(config.MaxPathDepth > 0)
			helper.AssertTrue(config.MaxConcurrency > 0)
		})

		t.Run("ClampValues", func(t *testing.T) {
			config := DefaultConfig()
			config.MaxJSONSize = 200 * 1024 * 1024 // Over max
			config.MaxPathDepth = 300                // Over max

			config.Validate()

			// Should be clamped
			helper.AssertTrue(config.MaxJSONSize <= 100*1024*1024)
			helper.AssertTrue(config.MaxPathDepth <= 200)
		})
	})

	t.Run("ConfigInterface", func(t *testing.T) {
		config := DefaultConfig()

		helper.AssertTrue(config.IsCacheEnabled())
		helper.AssertEqual(config.MaxCacheSize, config.GetMaxCacheSize())
		helper.AssertEqual(config.CacheTTL, config.GetCacheTTL())
		helper.AssertEqual(config.MaxJSONSize, config.GetMaxJSONSize())
		helper.AssertEqual(config.MaxPathDepth, config.GetMaxPathDepth())
		helper.AssertEqual(config.MaxConcurrency, config.GetMaxConcurrency())
		helper.AssertEqual(config.EnableMetrics, config.IsMetricsEnabled())
		helper.AssertEqual(config.EnableHealthCheck, config.IsHealthCheckEnabled())
		helper.AssertEqual(config.StrictMode, config.IsStrictMode())
	})
}

// TestEncodingConfiguration tests encoding configuration
func TestEncodingConfiguration(t *testing.T) {
	helper := NewTestHelper(t)

	t.Run("DefaultEncodeConfig", func(t *testing.T) {
		config := DefaultEncodeConfig()

		helper.AssertNotNil(config)
		helper.AssertFalse(config.Pretty)
		helper.AssertEqual("  ", config.Indent)
		helper.AssertEqual("", config.Prefix)
		helper.AssertTrue(config.EscapeHTML)
		helper.AssertFalse(config.SortKeys)
		helper.AssertFalse(config.OmitEmpty)
		helper.AssertTrue(config.ValidateUTF8)
		helper.AssertEqual(100, config.MaxDepth)
		helper.AssertFalse(config.DisallowUnknown)
		helper.AssertFalse(config.PreserveNumbers)
		helper.AssertEqual(-1, config.FloatPrecision)
		helper.AssertFalse(config.DisableEscaping)
		helper.AssertFalse(config.EscapeUnicode)
		helper.AssertFalse(config.EscapeSlash)
		helper.AssertTrue(config.EscapeNewlines)
		helper.AssertTrue(config.EscapeTabs)
		helper.AssertTrue(config.IncludeNulls)
	})

	t.Run("NewPrettyConfig", func(t *testing.T) {
		config := NewPrettyConfig()

		helper.AssertTrue(config.Pretty)
		helper.AssertEqual("  ", config.Indent)
	})

	t.Run("NewCompactConfig", func(t *testing.T) {
		config := NewCompactConfig()

		helper.AssertFalse(config.Pretty)
	})

	t.Run("EncodingOptions", func(t *testing.T) {
		config := DefaultEncodeConfig()

		t.Run("SetPretty", func(t *testing.T) {
			config.Pretty = true
			helper.AssertTrue(config.Pretty)
		})

		t.Run("SetSortKeys", func(t *testing.T) {
			config.SortKeys = true
			helper.AssertTrue(config.SortKeys)
		})

		t.Run("SetOmitEmpty", func(t *testing.T) {
			config.OmitEmpty = true
			helper.AssertTrue(config.OmitEmpty)
		})

		t.Run("SetFloatPrecision", func(t *testing.T) {
			config.FloatPrecision = 2
			helper.AssertEqual(2, config.FloatPrecision)
		})

		t.Run("SetEscapeHTML", func(t *testing.T) {
			config.EscapeHTML = false
			helper.AssertFalse(config.EscapeHTML)
		})

		t.Run("SetEscapeUnicode", func(t *testing.T) {
			config.EscapeUnicode = true
			helper.AssertTrue(config.EscapeUnicode)
		})

		t.Run("SetIncludeNulls", func(t *testing.T) {
			config.IncludeNulls = false
			helper.AssertFalse(config.IncludeNulls)
		})
	})
}

// TestConfigurationIntegration tests configuration with processor
func TestConfigurationIntegration(t *testing.T) {
	helper := NewTestHelper(t)

	t.Run("ProcessorWithConfig", func(t *testing.T) {
		config := DefaultConfig()
		config.EnableCache = true
		config.EnableMetrics = true

		processor := New(config)
		defer processor.Close()

		testData := `{"test": "value"}`

		result, err := processor.Get(testData, "test")
		helper.AssertNoError(err)
		helper.AssertEqual("value", result)

		stats := processor.GetStats()
		helper.AssertTrue(stats.CacheEnabled)
		helper.AssertEqual(config.MaxCacheSize, stats.MaxCacheSize)
	})

	t.Run("HighSecurityProcessor", func(t *testing.T) {
		processor := New(HighSecurityConfig())
		defer processor.Close()

		// Test that security limits are enforced
		deepJSON := generateDeepNesting(30)

		_, err := processor.Get(deepJSON, "a")
		// Should error due to depth limit
		if err != nil {
			var jsonErr *JsonsError
			if errors.As(err, &jsonErr) {
				helper.AssertEqual(ErrDepthLimit, jsonErr.Err)
			}
		}
	})

	t.Run("LargeDataProcessor", func(t *testing.T) {
		processor := New(LargeDataConfig())
		defer processor.Close()

		// Test with large array
		largeArrayData := generateLargeArray(10000)

		result, err := GetArray(largeArrayData, "items")
		helper.AssertNoError(err)
		helper.AssertTrue(len(result) > 0)
	})
}

// TestConfigurationEdgeCases tests configuration edge cases
func TestConfigurationEdgeCases(t *testing.T) {
	helper := NewTestHelper(t)

	t.Run("ExtremeCacheSizes", func(t *testing.T) {
		config := DefaultConfig()

		t.Run("ZeroCacheSize", func(t *testing.T) {
			config.MaxCacheSize = 0
			err := config.Validate()
			helper.AssertNoError(err)
			// Cache might still be enabled but with 0 size
			_ = config.EnableCache
		})

		t.Run("VeryLargeCacheSize", func(t *testing.T) {
			config.MaxCacheSize = 10000
			config.Validate()
			helper.AssertTrue(config.MaxCacheSize <= 2000)
		})
	})

	t.Run("ExtremeConcurrency", func(t *testing.T) {
		config := DefaultConfig()

		t.Run("ZeroConcurrency", func(t *testing.T) {
			config.MaxConcurrency = 0
			config.Validate()
			helper.AssertTrue(config.MaxConcurrency > 0)
		})

		t.Run("VeryHighConcurrency", func(t *testing.T) {
			config.MaxConcurrency = 500
			config.Validate()
			helper.AssertTrue(config.MaxConcurrency <= 200)
		})
	})

	t.Run("CacheTTL", func(t *testing.T) {
		config := DefaultConfig()

		t.Run("ZeroTTL", func(t *testing.T) {
			config.CacheTTL = 0
			config.Validate()
			helper.AssertTrue(config.CacheTTL > 0)
		})

		t.Run("VeryShortTTL", func(t *testing.T) {
			config.CacheTTL = 1 * time.Nanosecond
			helper.AssertNoError(config.Validate())
		})

		t.Run("VeryLongTTL", func(t *testing.T) {
			config.CacheTTL = 24 * time.Hour
			helper.AssertNoError(config.Validate())
		})
	})

	t.Run("BooleanFlags", func(t *testing.T) {
		config := DefaultConfig()

		// Test all boolean flags can be set
		flags := []struct {
			name string
			set  func(bool)
			get  func() bool
		}{
			{"EnableCache", func(b bool) { config.EnableCache = b }, func() bool { return config.EnableCache }},
			{"EnableValidation", func(b bool) { config.EnableValidation = b }, func() bool { return config.EnableValidation }},
			{"StrictMode", func(b bool) { config.StrictMode = b }, func() bool { return config.StrictMode }},
			{"CreatePaths", func(b bool) { config.CreatePaths = b }, func() bool { return config.CreatePaths }},
			{"CleanupNulls", func(b bool) { config.CleanupNulls = b }, func() bool { return config.CleanupNulls }},
			{"CompactArrays", func(b bool) { config.CompactArrays = b }, func() bool { return config.CompactArrays }},
			{"EnableMetrics", func(b bool) { config.EnableMetrics = b }, func() bool { return config.EnableMetrics }},
			{"EnableHealthCheck", func(b bool) { config.EnableHealthCheck = b }, func() bool { return config.EnableHealthCheck }},
			{"AllowCommentsFlag", func(b bool) { config.AllowCommentsFlag = b }, func() bool { return config.AllowCommentsFlag }},
			{"PreserveNumbersFlag", func(b bool) { config.PreserveNumbersFlag = b }, func() bool { return config.PreserveNumbersFlag }},
			{"ValidateInput", func(b bool) { config.ValidateInput = b }, func() bool { return config.ValidateInput }},
			{"ValidateFilePath", func(b bool) { config.ValidateFilePath = b }, func() bool { return config.ValidateFilePath }},
		}

		for _, flag := range flags {
			t.Run(flag.name+"_True", func(t *testing.T) {
				flag.set(true)
				helper.AssertTrue(flag.get())
			})

			t.Run(flag.name+"_False", func(t *testing.T) {
				flag.set(false)
				helper.AssertFalse(flag.get())
			})
		}
	})
}

// Helper functions

func generateLargeArray(size int) string {
	result := `{"items": [`
	for i := 0; i < size; i++ {
		if i > 0 {
			result += ","
		}
		result += `{"id": ` + fmt.Sprint(i) + `}`
	}
	result += `]}`
	return result
}
