package json

// LoadFromFile loads JSON data from a file with optional processor configuration
// Uses the default processor with support for ProcessorOptions such as security validation
func LoadFromFile(filePath string, opts ...*ProcessorOptions) (string, error) {
	return getDefaultProcessor().LoadFromFile(filePath, opts...)
}

// SaveToFile saves JSON data to a file with optional formatting
// This function accepts any Go value and converts it to JSON before saving.
//
// Special behavior for string and []byte inputs:
//   - If data is a JSON string, it will be parsed first to prevent double-encoding.
//   - If data is []byte containing JSON, it will be parsed first.
//   - This ensures that SaveToFile("file.json", `{"a":1}`) writes {"a":1} not "{\"a\":1}"
//
// Uses the default processor for security validation and encoding.
func SaveToFile(filePath string, data any, pretty ...bool) error {
	return getDefaultProcessor().SaveToFile(filePath, data, pretty...)
}

// MarshalToFile converts data to JSON and saves it to the specified file.
// This is a convenience function that combines Marshal and file writing operations.
// Uses the default processor for security validation and encoding.
//
// Parameters:
//   - path: file path where JSON will be saved (directories are created automatically)
//   - data: any Go value to be marshaled to JSON
//   - pretty: optional parameter - true for formatted JSON, false for compact (default: false)
//
// Returns error if marshaling fails or file cannot be written.
//
// Special behavior for string and []byte inputs:
//   - If data is a JSON string, it will be parsed first to prevent double-encoding.
//   - If data is []byte containing JSON, it will be parsed first.
func MarshalToFile(path string, data any, pretty ...bool) error {
	return getDefaultProcessor().MarshalToFile(path, data, pretty...)
}

// UnmarshalFromFile reads JSON from a file and unmarshals it into v.
// This is a convenience function that combines file reading and unmarshalling.
// Uses the default processor for security validation and decoding.
//
// Parameters:
//   - path: file path to read JSON from
//   - v: pointer to the target variable where JSON will be unmarshaled
//
// Returns error if file reading fails or JSON cannot be unmarshaled.
func UnmarshalFromFile(path string, v any) error {
	return getDefaultProcessor().UnmarshalFromFile(path, v)
}
