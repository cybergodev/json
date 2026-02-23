package json

// ProcessBatch processes multiple JSON operations in a single batch.
// This is more efficient than processing each operation individually.
func ProcessBatch(operations []BatchOperation, opts ...*ProcessorOptions) ([]BatchResult, error) {
	return getDefaultProcessor().ProcessBatch(operations, opts...)
}

// WarmupCache pre-warms the cache for frequently accessed paths.
// This can improve performance for subsequent operations on the same JSON.
func WarmupCache(jsonStr string, paths []string, opts ...*ProcessorOptions) (*WarmupResult, error) {
	return getDefaultProcessor().WarmupCache(jsonStr, paths, opts...)
}

// ClearCache clears the processor's internal cache.
func ClearCache() {
	getDefaultProcessor().ClearCache()
}

// GetStats returns statistics about the default processor.
func GetStats() Stats {
	return getDefaultProcessor().GetStats()
}

// GetHealthStatus returns the health status of the default processor.
func GetHealthStatus() HealthStatus {
	return getDefaultProcessor().GetHealthStatus()
}

// EncodeStream encodes multiple values as a JSON stream.
func EncodeStream(values any, pretty bool, opts ...*ProcessorOptions) (string, error) {
	return getDefaultProcessor().EncodeStream(values, pretty, opts...)
}

// EncodeBatch encodes multiple key-value pairs as a JSON object.
func EncodeBatch(pairs map[string]any, pretty bool, opts ...*ProcessorOptions) (string, error) {
	return getDefaultProcessor().EncodeBatch(pairs, pretty, opts...)
}

// EncodeFields encodes specific fields from a struct or map.
func EncodeFields(value any, fields []string, pretty bool, opts ...*ProcessorOptions) (string, error) {
	return getDefaultProcessor().EncodeFields(value, fields, pretty, opts...)
}

// mergeOptionsWithOverride creates a new options with overrides applied
func mergeOptionsWithOverride(opts []*ProcessorOptions, override func(*ProcessorOptions)) *ProcessorOptions {
	var result *ProcessorOptions
	if len(opts) > 0 && opts[0] != nil {
		result = opts[0].Clone()
	} else {
		result = DefaultOptions()
	}
	override(result)
	return result
}
