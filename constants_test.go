package json

import (
	"testing"
	"time"
)

func TestConstants(t *testing.T) {
	helper := NewTestHelper(t)

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
}
