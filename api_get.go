package json

// Get retrieves a value from JSON at the specified path
func Get(jsonStr, path string, opts ...*ProcessorOptions) (any, error) {
	return getDefaultProcessor().Get(jsonStr, path, opts...)
}

// GetTyped retrieves a typed value from JSON at the specified path
func GetTyped[T any](jsonStr, path string, opts ...*ProcessorOptions) (T, error) {
	return GetTypedWithProcessor[T](getDefaultProcessor(), jsonStr, path, opts...)
}

// GetString retrieves a string value from JSON at the specified path
func GetString(jsonStr, path string, opts ...*ProcessorOptions) (string, error) {
	return getDefaultProcessor().GetString(jsonStr, path, opts...)
}

// GetInt retrieves an int value from JSON at the specified path
func GetInt(jsonStr, path string, opts ...*ProcessorOptions) (int, error) {
	return getDefaultProcessor().GetInt(jsonStr, path, opts...)
}

// GetFloat64 retrieves a float64 value from JSON at the specified path
func GetFloat64(jsonStr, path string, opts ...*ProcessorOptions) (float64, error) {
	return getDefaultProcessor().GetFloat64(jsonStr, path, opts...)
}

// GetBool retrieves a bool value from JSON at the specified path
func GetBool(jsonStr, path string, opts ...*ProcessorOptions) (bool, error) {
	return getDefaultProcessor().GetBool(jsonStr, path, opts...)
}

// GetArray retrieves an array value from JSON at the specified path
func GetArray(jsonStr, path string, opts ...*ProcessorOptions) ([]any, error) {
	return getDefaultProcessor().GetArray(jsonStr, path, opts...)
}

// GetObject retrieves an object value from JSON at the specified path
func GetObject(jsonStr, path string, opts ...*ProcessorOptions) (map[string]any, error) {
	return getDefaultProcessor().GetObject(jsonStr, path, opts...)
}

// GetWithDefault retrieves a value from JSON at the specified path with a default fallback
func GetWithDefault(jsonStr, path string, defaultValue any, opts ...*ProcessorOptions) any {
	return getDefaultProcessor().GetWithDefault(jsonStr, path, defaultValue, opts...)
}

// GetTypedWithDefault retrieves a typed value from JSON at the specified path with a default fallback
func GetTypedWithDefault[T any](jsonStr, path string, defaultValue T, opts ...*ProcessorOptions) T {
	result, err := GetTyped[T](jsonStr, path, opts...)
	if err != nil || isZeroValue(result) {
		return defaultValue
	}
	return result
}

// GetStringWithDefault retrieves a string value from JSON at the specified path with a default fallback
func GetStringWithDefault(jsonStr, path, defaultValue string, opts ...*ProcessorOptions) string {
	result, err := GetString(jsonStr, path, opts...)
	if err != nil {
		return defaultValue
	}
	return result
}

// GetIntWithDefault retrieves an int value from JSON at the specified path with a default fallback
func GetIntWithDefault(jsonStr, path string, defaultValue int, opts ...*ProcessorOptions) int {
	result, err := GetInt(jsonStr, path, opts...)
	if err != nil {
		return defaultValue
	}
	return result
}

// GetFloat64WithDefault retrieves a float64 value from JSON at the specified path with a default fallback
func GetFloat64WithDefault(jsonStr, path string, defaultValue float64, opts ...*ProcessorOptions) float64 {
	result, err := GetFloat64(jsonStr, path, opts...)
	if err != nil {
		return defaultValue
	}
	return result
}

// GetBoolWithDefault retrieves a bool value from JSON at the specified path with a default fallback
func GetBoolWithDefault(jsonStr, path string, defaultValue bool, opts ...*ProcessorOptions) bool {
	result, err := GetBool(jsonStr, path, opts...)
	if err != nil {
		return defaultValue
	}
	return result
}

// GetArrayWithDefault retrieves an array value from JSON at the specified path with a default fallback
func GetArrayWithDefault(jsonStr, path string, defaultValue []any, opts ...*ProcessorOptions) []any {
	result, err := GetArray(jsonStr, path, opts...)
	if err != nil {
		return defaultValue
	}
	return result
}

// GetObjectWithDefault retrieves an object value from JSON at the specified path with a default fallback
func GetObjectWithDefault(jsonStr, path string, defaultValue map[string]any, opts ...*ProcessorOptions) map[string]any {
	result, err := GetObject(jsonStr, path, opts...)
	if err != nil {
		return defaultValue
	}
	return result
}

// GetMultiple retrieves multiple values from JSON at the specified paths
func GetMultiple(jsonStr string, paths []string, opts ...*ProcessorOptions) (map[string]any, error) {
	return getDefaultProcessor().GetMultiple(jsonStr, paths, opts...)
}

// isZeroValue checks if a value is the zero value for its type
func isZeroValue(v any) bool {
	if v == nil {
		return true
	}
	switch val := v.(type) {
	case string:
		return val == ""
	case int:
		return val == 0
	case int64:
		return val == 0
	case float64:
		return val == 0
	case bool:
		return !val
	case []any:
		return len(val) == 0
	case map[string]any:
		return len(val) == 0
	default:
		return false
	}
}
