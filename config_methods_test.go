package json

import (
	"testing"
)

// TestConfigMethods tests Config getter methods
func TestConfigMethods(t *testing.T) {
	helper := NewTestHelper(t)

	t.Run("Config Getters", func(t *testing.T) {
		config := DefaultConfig()

		helper.AssertTrue(config.IsCacheEnabled(), "Cache should be enabled")
		helper.AssertTrue(config.GetMaxCacheSize() > 0, "Max cache size should be positive")
		helper.AssertTrue(config.GetCacheTTL() > 0, "Cache TTL should be positive")
		helper.AssertTrue(config.GetMaxJSONSize() > 0, "Max JSON size should be positive")
		helper.AssertTrue(config.GetMaxPathDepth() > 0, "Max path depth should be positive")
		helper.AssertTrue(config.GetMaxConcurrency() > 0, "Max concurrency should be positive")
		_ = config.IsMetricsEnabled()
		_ = config.IsHealthCheckEnabled()
		_ = config.IsStrictMode()
		_ = config.AllowComments()
		_ = config.PreserveNumbers()
		_ = config.ShouldCreatePaths()
		_ = config.ShouldCleanupNulls()
		_ = config.ShouldCompactArrays()
		_ = config.ShouldValidateInput()
		helper.AssertTrue(config.GetMaxNestingDepth() >= 0, "Max nesting depth should be non-negative")
		_ = config.IsRateLimitEnabled()
		helper.AssertTrue(config.GetRateLimitPerSec() >= 0, "Rate limit should be non-negative")
		_ = config.ShouldValidateFilePath()
		_ = config.AreResourcePoolsEnabled()
		helper.AssertTrue(config.GetMaxPoolSize() >= 0, "Max pool size should be non-negative")
		helper.AssertTrue(config.GetPoolCleanupInterval() >= 0, "Pool cleanup interval should be non-negative")

		limits := config.GetSecurityLimits()
		helper.AssertNotNil(limits, "Security limits should not be nil")
	})

	t.Run("Config Presets", func(t *testing.T) {
		config := DefaultProcessorConfig()
		helper.AssertNotNil(config, "Default processor config should not be nil")

		highSec := HighSecurityConfig()
		helper.AssertNotNil(highSec, "High security config should not be nil")

		largeDat := LargeDataConfig()
		helper.AssertNotNil(largeDat, "Large data config should not be nil")
	})

	t.Run("Schema Methods", func(t *testing.T) {
		schema := DefaultSchema()

		schema.SetMinLength(5)
		helper.AssertTrue(schema.HasMinLength(), "HasMinLength should return true")

		schema.SetMaxLength(100)
		helper.AssertTrue(schema.HasMaxLength(), "HasMaxLength should return true")

		schema.SetMinimum(0.0)
		helper.AssertTrue(schema.HasMinimum(), "HasMinimum should return true")

		schema.SetMaximum(1000.0)
		helper.AssertTrue(schema.HasMaximum(), "HasMaximum should return true")

		schema.SetMinItems(1)
		helper.AssertTrue(schema.HasMinItems(), "HasMinItems should return true")

		schema.SetMaxItems(50)
		helper.AssertTrue(schema.HasMaxItems(), "HasMaxItems should return true")

		schema.SetExclusiveMinimum(true)
		schema.SetExclusiveMaximum(true)
	})

	t.Run("EncodeConfig Presets", func(t *testing.T) {
		config := DefaultEncodeConfig()
		helper.AssertNotNil(config, "Default encode config should not be nil")

		pretty := NewPrettyConfig()
		helper.AssertNotNil(pretty, "Pretty config should not be nil")
		helper.AssertTrue(pretty.Pretty, "Pretty should be enabled")

		compact := NewCompactConfig()
		helper.AssertNotNil(compact, "Compact config should not be nil")

		readable := NewReadableConfig()
		helper.AssertNotNil(readable, "Readable config should not be nil")

		webSafe := NewWebSafeConfig()
		helper.AssertNotNil(webSafe, "Web safe config should not be nil")

		clean := NewCleanConfig()
		helper.AssertNotNil(clean, "Clean config should not be nil")
	})

	t.Run("ResourceMonitor", func(t *testing.T) {
		monitor := NewResourceMonitor()

		monitor.RecordAllocation(1024)
		monitor.RecordDeallocation(512)
		monitor.RecordPoolHit()
		monitor.RecordPoolMiss()
		monitor.RecordPoolEviction()
		monitor.RecordOperation(1000)

		leaks := monitor.CheckForLeaks()
		_ = leaks

		stats := monitor.GetStats()
		helper.AssertNotNil(stats, "Stats should not be nil")

		monitor.Reset()

		eff := monitor.GetMemoryEfficiency()
		helper.AssertTrue(eff >= 0 && eff <= 100, "Efficiency should be 0-100")

		poolEff := monitor.GetPoolEfficiency()
		helper.AssertTrue(poolEff >= 0 && poolEff <= 100, "Pool efficiency should be 0-100")
	})

	t.Run("ValidateConfig", func(t *testing.T) {
		config := DefaultConfig()
		err := ValidateConfig(config)
		helper.AssertNoError(err, "Default config should be valid")

		invalidConfig := DefaultConfig()
		invalidConfig.MaxCacheSize = -1
		err = ValidateConfig(invalidConfig)
		helper.AssertError(err, "Negative cache size should be invalid")
	})

	t.Run("ValidateOptions", func(t *testing.T) {
		opts := DefaultOptions()
		err := ValidateOptions(opts)
		helper.AssertNoError(err, "Default options should be valid")
	})
}
