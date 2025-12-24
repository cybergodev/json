package json

import (
	"testing"
	"time"
)

// TestConfigConstants consolidates configuration and constants validation tests
// Replaces: config_methods_test.go, constants_test.go
func TestConfigConstants(t *testing.T) {
	helper := NewTestHelper(t)

	t.Run("ConfigGetters", func(t *testing.T) {
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
		_ = config.ShouldValidateFilePath()

		limits := config.GetSecurityLimits()
		helper.AssertNotNil(limits, "Security limits should not be nil")
	})

	t.Run("ConfigPresets", func(t *testing.T) {
		config := DefaultProcessorConfig()
		helper.AssertNotNil(config, "Default processor config should not be nil")

		highSec := HighSecurityConfig()
		helper.AssertNotNil(highSec, "High security config should not be nil")

		largeDat := LargeDataConfig()
		helper.AssertNotNil(largeDat, "Large data config should not be nil")
	})

	t.Run("SchemaOperations", func(t *testing.T) {
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

	t.Run("EncodeConfigPresets", func(t *testing.T) {
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

		// Test clone
		cloned := config.Clone()
		helper.AssertNotNil(cloned, "Cloned config should not be nil")
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

	t.Run("ConfigValidation", func(t *testing.T) {
		config := DefaultConfig()
		err := ValidateConfig(config)
		helper.AssertNoError(err, "Default config should be valid")

		invalidConfig := DefaultConfig()
		invalidConfig.MaxCacheSize = -1
		err = ValidateConfig(invalidConfig)
		helper.AssertError(err, "Negative cache size should be invalid")

		opts := DefaultOptions()
		err = ValidateOptions(opts)
		helper.AssertNoError(err, "Default options should be valid")
	})

	t.Run("BufferAndPoolSizes", func(t *testing.T) {
		helper.AssertTrue(DefaultBufferSize > 0, "DefaultBufferSize should be positive")
		helper.AssertTrue(MaxPoolBufferSize > MinPoolBufferSize, "MaxPoolBufferSize should be greater than MinPoolBufferSize")
		helper.AssertTrue(DefaultPathSegmentCap > 0, "DefaultPathSegmentCap should be positive")
		helper.AssertTrue(MaxPathSegmentCap > DefaultPathSegmentCap, "MaxPathSegmentCap should be greater than DefaultPathSegmentCap")
	})

	t.Run("CacheSizes", func(t *testing.T) {
		helper.AssertTrue(DefaultCacheSize > 0, "DefaultCacheSize should be positive")
		helper.AssertTrue(MaxCacheEntries > DefaultCacheSize, "MaxCacheEntries should be greater than DefaultCacheSize")
		helper.AssertEqual(MaxCacheEntries/2, CacheCleanupKeepSize, "CacheCleanupKeepSize should be half of MaxCacheEntries")
	})

	t.Run("OperationLimits", func(t *testing.T) {
		helper.AssertTrue(InvalidArrayIndex < 0, "InvalidArrayIndex should be negative")
		helper.AssertTrue(DefaultMaxJSONSize > 0, "DefaultMaxJSONSize should be positive")
		helper.AssertTrue(DefaultMaxNestingDepth > 0, "DefaultMaxNestingDepth should be positive")
		helper.AssertTrue(DefaultMaxObjectKeys > 0, "DefaultMaxObjectKeys should be positive")
		helper.AssertTrue(DefaultMaxArrayElements > 0, "DefaultMaxArrayElements should be positive")
		helper.AssertTrue(DefaultMaxPathDepth > 0, "DefaultMaxPathDepth should be positive")
		helper.AssertTrue(DefaultMaxBatchSize > 0, "DefaultMaxBatchSize should be positive")
		helper.AssertTrue(DefaultMaxConcurrency > 0, "DefaultMaxConcurrency should be positive")
	})

	t.Run("TimingAndIntervals", func(t *testing.T) {
		helper.AssertTrue(MemoryPressureCheckInterval > 0, "MemoryPressureCheckInterval should be positive")
		helper.AssertTrue(PoolResetInterval > PoolResetIntervalPressure, "PoolResetInterval should be greater than PoolResetIntervalPressure")
		helper.AssertTrue(CacheCleanupInterval > 0, "CacheCleanupInterval should be positive")
		helper.AssertTrue(DeadlockCheckInterval > 0, "DeadlockCheckInterval should be positive")
		helper.AssertTrue(DeadlockThreshold > 0, "DeadlockThreshold should be positive")
		helper.AssertTrue(SlowOperationThreshold > 0, "SlowOperationThreshold should be positive")
	})

	t.Run("RetryAndTimeout", func(t *testing.T) {
		helper.AssertTrue(MaxRetries > 0, "MaxRetries should be positive")
		helper.AssertTrue(BaseRetryDelay > 0, "BaseRetryDelay should be positive")
		helper.AssertTrue(DefaultOperationTimeout > 0, "DefaultOperationTimeout should be positive")
		helper.AssertTrue(AcquireSlotRetryDelay > 0, "AcquireSlotRetryDelay should be positive")
		helper.AssertTrue(DefaultOperationTimeout > BaseRetryDelay*time.Duration(MaxRetries), "DefaultOperationTimeout should allow for retries")
	})

	t.Run("PathValidation", func(t *testing.T) {
		helper.AssertTrue(MaxPathLength > 0, "MaxPathLength should be positive")
		helper.AssertTrue(MaxSegmentLength > 0, "MaxSegmentLength should be positive")
		helper.AssertTrue(MaxExtractionDepth > 0, "MaxExtractionDepth should be positive")
		helper.AssertTrue(MaxConsecutiveColons > 0, "MaxConsecutiveColons should be positive")
		helper.AssertTrue(MaxConsecutiveBrackets > 0, "MaxConsecutiveBrackets should be positive")
	})

	t.Run("SecurityConstants", func(t *testing.T) {
		helper.AssertTrue(MaxSecurityValidationSize > 0, "MaxSecurityValidationSize should be positive")
		helper.AssertTrue(MaxAllowedNestingDepth > 0, "MaxAllowedNestingDepth should be positive")
		helper.AssertTrue(MaxAllowedObjectKeys > 0, "MaxAllowedObjectKeys should be positive")
		helper.AssertTrue(MaxAllowedArrayElements > 0, "MaxAllowedArrayElements should be positive")
		helper.AssertEqual(DefaultMaxSecuritySize, MaxSecurityValidationSize, "Security size constants should match")
		helper.AssertEqual(DefaultMaxNestingDepth, MaxAllowedNestingDepth, "Nesting depth constants should match")
	})

	t.Run("ErrorCodes", func(t *testing.T) {
		errorCodes := []string{
			ErrCodeInvalidJSON,
			ErrCodePathNotFound,
			ErrCodeTypeMismatch,
			ErrCodeSizeLimit,
			ErrCodeDepthLimit,
			ErrCodeSecurityViolation,
			ErrCodeRateLimit,
			ErrCodeTimeout,
			ErrCodeConcurrencyLimit,
			ErrCodeProcessorClosed,
		}

		for _, code := range errorCodes {
			helper.AssertTrue(len(code) > 0, "Error code should not be empty")
			helper.AssertTrue(code[:4] == "ERR_", "Error code should start with ERR_")
		}

		// Check uniqueness
		seen := make(map[string]bool)
		for _, code := range errorCodes {
			helper.AssertFalse(seen[code], "Error code %s should be unique", code)
			seen[code] = true
		}
	})

	t.Run("CacheTTL", func(t *testing.T) {
		helper.AssertTrue(DefaultCacheTTL > 0, "DefaultCacheTTL should be positive")
		helper.AssertTrue(DefaultCacheTTL >= time.Minute, "DefaultCacheTTL should be at least 1 minute")
	})

	t.Run("GlobalProcessorOperations", func(t *testing.T) {
		// Test global processor management
		SetGlobalProcessor(New(DefaultConfig()))
		ShutdownGlobalProcessor()
	})
}
