package json

// Delete deletes a value from JSON at the specified path
func Delete(jsonStr, path string, opts ...*ProcessorOptions) (string, error) {
	return getDefaultProcessor().Delete(jsonStr, path, opts...)
}

// DeleteWithCleanNull removes a value from JSON and cleans up null values
func DeleteWithCleanNull(jsonStr, path string, opts ...*ProcessorOptions) (string, error) {
	return getDefaultProcessor().DeleteWithCleanNull(jsonStr, path, opts...)
}
