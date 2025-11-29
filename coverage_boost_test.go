package json

import (
	"strings"
	"testing"
)

// TestCoverageBoost adds tests for uncovered functions
func TestCoverageBoost(t *testing.T) {
	helper := NewTestHelper(t)

	t.Run("Config Methods", func(t *testing.T) {
		config := DefaultConfig()
		_ = config.GetMaxJSONSize()
		_ = config.GetMaxPathDepth()
		_ = config.GetMaxConcurrency()
		_ = config.IsMetricsEnabled()
		_ = config.IsHealthCheckEnabled()
		_ = config.IsStrictMode()
		_ = config.AllowComments()
		_ = config.PreserveNumbers()
		_ = config.ShouldCreatePaths()
		_ = config.ShouldCleanupNulls()
		_ = config.ShouldCompactArrays()
		_ = config.ShouldValidateInput()
		_ = config.GetMaxNestingDepth()
		_ = config.IsRateLimitEnabled()
		_ = config.GetRateLimitPerSec()
		_ = config.ShouldValidateFilePath()
		_ = config.AreResourcePoolsEnabled()
		_ = config.GetMaxPoolSize()
		_ = config.GetPoolCleanupInterval()
		limits := config.GetSecurityLimits()
		helper.AssertNotNil(limits, "Security limits should not be nil")
	})

	t.Run("Config Presets", func(t *testing.T) {
		_ = DefaultProcessorConfig()
		_ = HighSecurityConfig()
		_ = LargeDataConfig()
		_ = DefaultOptions()
	})

	t.Run("Schema Methods", func(t *testing.T) {
		schema := DefaultSchema()
		schema.SetMinLength(5)
		helper.AssertTrue(schema.HasMinLength(), "HasMinLength should work")
		schema.SetMaxLength(100)
		helper.AssertTrue(schema.HasMaxLength(), "HasMaxLength should work")
		schema.SetMinimum(0.0)
		helper.AssertTrue(schema.HasMinimum(), "HasMinimum should work")
		schema.SetMaximum(1000.0)
		helper.AssertTrue(schema.HasMaximum(), "HasMaximum should work")
		schema.SetMinItems(1)
		helper.AssertTrue(schema.HasMinItems(), "HasMinItems should work")
		schema.SetMaxItems(50)
		helper.AssertTrue(schema.HasMaxItems(), "HasMaxItems should work")
		schema.SetExclusiveMinimum(true)
		schema.SetExclusiveMaximum(true)
	})

	t.Run("EncodeConfig Presets", func(t *testing.T) {
		_ = NewCompactConfig()
		_ = NewReadableConfig()
		_ = NewWebSafeConfig()
		_ = NewCleanConfig()
	})

	t.Run("ResourceMonitor", func(t *testing.T) {
		monitor := NewResourceMonitor()
		monitor.RecordAllocation(1024)
		monitor.RecordPoolMiss()
		monitor.RecordOperation(1000)
		_ = monitor.CheckForLeaks()
		_ = monitor.GetStats()
		monitor.Reset()
		_ = monitor.GetMemoryEfficiency()
		_ = monitor.GetPoolEfficiency()
	})

	t.Run("GetArray", func(t *testing.T) {
		jsonStr := `{"items":[1,2,3]}`
		arr, err := GetArray(jsonStr, "items")
		helper.AssertNoError(err, "GetArray should work")
		helper.AssertEqual(len(arr), 3, "Array length should be 3")
	})

	t.Run("GetObject", func(t *testing.T) {
		jsonStr := `{"user":{"name":"test"}}`
		obj, err := GetObject(jsonStr, "user")
		helper.AssertNoError(err, "GetObject should work")
		helper.AssertNotNil(obj, "Object should not be nil")
	})

	t.Run("GetWithDefault", func(t *testing.T) {
		jsonStr := `{"value":"test"}`
		val := GetWithDefault(jsonStr, "missing", "default")
		helper.AssertEqual(val, "default", "Should return default")
	})

	t.Run("GetTypedWithDefault", func(t *testing.T) {
		jsonStr := `{"value":42}`
		val := GetTypedWithDefault[int](jsonStr, "missing", 99)
		helper.AssertEqual(val, 99, "Should return default")
	})

	t.Run("GetStringWithDefault", func(t *testing.T) {
		jsonStr := `{"value":"test"}`
		val := GetStringWithDefault(jsonStr, "missing", "default")
		helper.AssertEqual(val, "default", "Should return default")
	})

	t.Run("GetIntWithDefault", func(t *testing.T) {
		jsonStr := `{"value":42}`
		val := GetIntWithDefault(jsonStr, "missing", 99)
		helper.AssertEqual(val, 99, "Should return default")
	})

	t.Run("GetFloat64WithDefault", func(t *testing.T) {
		jsonStr := `{"value":3.14}`
		val := GetFloat64WithDefault(jsonStr, "missing", 9.99)
		helper.AssertEqual(val, 9.99, "Should return default")
	})

	t.Run("GetBoolWithDefault", func(t *testing.T) {
		jsonStr := `{"value":true}`
		val := GetBoolWithDefault(jsonStr, "missing", true)
		helper.AssertTrue(val, "Should return default")
	})

	t.Run("GetArrayWithDefault", func(t *testing.T) {
		jsonStr := `{"items":[1,2,3]}`
		arr := GetArrayWithDefault(jsonStr, "missing", []any{99})
		helper.AssertEqual(len(arr), 1, "Should return default")
	})

	t.Run("GetObjectWithDefault", func(t *testing.T) {
		jsonStr := `{"user":{"name":"test"}}`
		obj := GetObjectWithDefault(jsonStr, "missing", map[string]any{"default": true})
		helper.AssertEqual(obj["default"], true, "Should return default")
	})

	t.Run("SetMultiple", func(t *testing.T) {
		jsonStr := `{"name":"test"}`
		updates := map[string]any{"age": 30}
		result, err := SetMultiple(jsonStr, updates)
		helper.AssertNoError(err, "SetMultiple should work")
		helper.AssertTrue(len(result) > 0, "Result should not be empty")
	})

	t.Run("SetMultipleWithAdd", func(t *testing.T) {
		jsonStr := `{"name":"test"}`
		updates := map[string]any{"age": 30}
		result, err := SetMultipleWithAdd(jsonStr, updates)
		helper.AssertNoError(err, "SetMultipleWithAdd should work")
		helper.AssertTrue(len(result) > 0, "Result should not be empty")
	})

	t.Run("DeleteWithCleanNull", func(t *testing.T) {
		jsonStr := `{"name":"test","age":30}`
		result, err := DeleteWithCleanNull(jsonStr, "age")
		helper.AssertNoError(err, "DeleteWithCleanNull should work")
		helper.AssertFalse(strings.Contains(result, "age"), "Should not contain deleted field")
	})

	t.Run("MarshalIndent", func(t *testing.T) {
		data := map[string]any{"name": "test"}
		result, err := MarshalIndent(data, "", "  ")
		helper.AssertNoError(err, "MarshalIndent should work")
		helper.AssertTrue(len(result) > 0, "Result should not be empty")
	})

	t.Run("EncodePretty", func(t *testing.T) {
		data := map[string]any{"name": "test"}
		result, err := EncodePretty(data)
		helper.AssertNoError(err, "EncodePretty should work")
		helper.AssertTrue(strings.Contains(result, "\n"), "Should be pretty")
	})

	t.Run("EncodeCompact", func(t *testing.T) {
		data := map[string]any{"name": "test"}
		result, err := EncodeCompact(data)
		helper.AssertNoError(err, "EncodeCompact should work")
		helper.AssertFalse(strings.Contains(result, "\n"), "Should be compact")
	})

	t.Run("FormatPretty", func(t *testing.T) {
		jsonStr := `{"name":"test"}`
		result, err := FormatPretty(jsonStr)
		helper.AssertNoError(err, "FormatPretty should work")
		helper.AssertTrue(strings.Contains(result, "\n"), "Should be pretty")
	})

	t.Run("FormatCompact", func(t *testing.T) {
		jsonStr := `{"name": "test"}`
		result, err := FormatCompact(jsonStr)
		helper.AssertNoError(err, "FormatCompact should work")
		helper.AssertFalse(strings.Contains(result, "\n"), "Should be compact")
	})

	t.Run("Processor Lifecycle", func(t *testing.T) {
		processor := New(DefaultConfig())
		helper.AssertFalse(processor.IsClosed(), "Should not be closed")
		processor.Close()
		helper.AssertTrue(processor.IsClosed(), "Should be closed")
	})

	t.Run("Processor Stats", func(t *testing.T) {
		processor := New(DefaultConfig())
		defer processor.Close()
		stats := processor.GetStats()
		helper.AssertNotNil(stats, "Stats should not be nil")
	})

	t.Run("Processor Config", func(t *testing.T) {
		processor := New(DefaultConfig())
		defer processor.Close()
		config := processor.GetConfig()
		helper.AssertNotNil(config, "Config should not be nil")
	})

	t.Run("Processor SetLogger", func(t *testing.T) {
		processor := New(DefaultConfig())
		defer processor.Close()
		processor.SetLogger(nil)
	})

	t.Run("Global Processor", func(t *testing.T) {
		SetGlobalProcessor(New(DefaultConfig()))
		ShutdownGlobalProcessor()
	})

	t.Run("EncodeConfig Clone", func(t *testing.T) {
		config := DefaultEncodeConfig()
		cloned := config.Clone()
		helper.AssertNotNil(cloned, "Cloned config should not be nil")
	})
}
