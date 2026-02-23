package json

// Set sets a value in JSON at the specified path
// Returns:
//   - On success: modified JSON string and nil error
//   - On failure: original unmodified JSON string and error information
func Set(jsonStr, path string, value any, opts ...*ProcessorOptions) (string, error) {
	return getDefaultProcessor().Set(jsonStr, path, value, opts...)
}

// SetWithAdd sets a value with automatic path creation
// Returns:
//   - On success: modified JSON string and nil error
//   - On failure: original unmodified JSON string and error information
func SetWithAdd(jsonStr, path string, value any) (string, error) {
	opts := &ProcessorOptions{CreatePaths: true}
	return Set(jsonStr, path, value, opts)
}

// SetMultiple sets multiple values using a map of path-value pairs
func SetMultiple(jsonStr string, updates map[string]any, opts ...*ProcessorOptions) (string, error) {
	return getDefaultProcessor().SetMultiple(jsonStr, updates, opts...)
}

// SetMultipleWithAdd sets multiple values with automatic path creation
func SetMultipleWithAdd(jsonStr string, updates map[string]any, opts ...*ProcessorOptions) (string, error) {
	createOpts := mergeOptionsWithOverride(opts, func(o *ProcessorOptions) {
		o.CreatePaths = true
	})
	return getDefaultProcessor().SetMultiple(jsonStr, updates, createOpts)
}
