package json

import (
	"bytes"
	"fmt"
	"os"
)

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
// Returns the default value only when an error occurs (e.g., path not found)
// Valid zero values (0, false, "") are returned as-is
func GetTypedWithDefault[T any](jsonStr, path string, defaultValue T, opts ...*ProcessorOptions) T {
	result, err := GetTyped[T](jsonStr, path, opts...)
	if err != nil {
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

// Delete deletes a value from JSON at the specified path
func Delete(jsonStr, path string, opts ...*ProcessorOptions) (string, error) {
	return getDefaultProcessor().Delete(jsonStr, path, opts...)
}

// DeleteWithCleanNull removes a value from JSON and cleans up null values
func DeleteWithCleanNull(jsonStr, path string, opts ...*ProcessorOptions) (string, error) {
	return getDefaultProcessor().DeleteWithCleanNull(jsonStr, path, opts...)
}

// Marshal returns the JSON encoding of v.
// This function is 100% compatible with encoding/json.Marshal.
func Marshal(v any) ([]byte, error) {
	return getDefaultProcessor().Marshal(v)
}

// Unmarshal parses the JSON-encoded data and stores the result in v.
// This function is 100% compatible with encoding/json.Unmarshal.
func Unmarshal(data []byte, v any) error {
	return getDefaultProcessor().Unmarshal(data, v)
}

// MarshalIndent is like Marshal but applies indentation to format the output.
// This function is 100% compatible with encoding/json.MarshalIndent.
func MarshalIndent(v any, prefix, indent string) ([]byte, error) {
	return getDefaultProcessor().MarshalIndent(v, prefix, indent)
}

// Compact appends to dst the JSON-encoded src with insignificant space characters elided.
// This function is 100% compatible with encoding/json.Compact.
func Compact(dst *bytes.Buffer, src []byte) error {
	compacted, err := FormatCompact(string(src))
	if err != nil {
		return err
	}
	dst.WriteString(compacted)
	return nil
}

// Indent appends to dst an indented form of the JSON-encoded src.
// This function is 100% compatible with encoding/json.Indent.
func Indent(dst *bytes.Buffer, src []byte, prefix, indent string) error {
	processor := getDefaultProcessor()
	result, err := processor.FormatPretty(string(src))
	if err != nil {
		return err
	}
	dst.WriteString(result)
	return nil
}

// HTMLEscape appends to dst the JSON-encoded src with <, >, &, U+2028, and U+2029 characters escaped.
// This function is 100% compatible with encoding/json.HTMLEscape.
func HTMLEscape(dst *bytes.Buffer, src []byte) {
	// Use standard library compatible HTML escaping
	result := htmlEscape(string(src))
	dst.WriteString(result)
}

// htmlEscape performs HTML escaping on JSON string
// Compatible with encoding/json: escapes <, >, &, U+2028, U+2029
func htmlEscape(s string) string {
	var buf bytes.Buffer
	buf.Grow(len(s))
	for _, r := range s {
		switch r {
		case '<':
			buf.WriteString("\\u003c")
		case '>':
			buf.WriteString("\\u003e")
		case '&':
			buf.WriteString("\\u0026")
		case '\u2028': // Line Separator - required for JSON-in-JS compatibility
			buf.WriteString("\\u2028")
		case '\u2029': // Paragraph Separator - required for JSON-in-JS compatibility
			buf.WriteString("\\u2029")
		default:
			buf.WriteRune(r)
		}
	}
	return buf.String()
}

// CompactBuffer is an alias for Compact for buffer operations
func CompactBuffer(dst *bytes.Buffer, src []byte, opts ...*ProcessorOptions) error {
	compacted, err := FormatCompact(string(src), opts...)
	if err != nil {
		return err
	}
	dst.WriteString(compacted)
	return nil
}

// IndentBuffer is an alias for Indent for buffer operations
func IndentBuffer(dst *bytes.Buffer, src []byte, prefix, indent string, opts ...*ProcessorOptions) error {
	result, err := FormatPretty(string(src), opts...)
	if err != nil {
		return err
	}
	dst.WriteString(result)
	return nil
}

// HTMLEscapeBuffer is an alias for HTMLEscape for buffer operations
func HTMLEscapeBuffer(dst *bytes.Buffer, src []byte, opts ...*ProcessorOptions) {
	result := htmlEscape(string(src))
	dst.WriteString(result)
}

// Encode converts any Go value to JSON string
func Encode(value any, config ...*EncodeConfig) (string, error) {
	var cfg *EncodeConfig
	if len(config) > 0 {
		cfg = config[0]
	}
	return getDefaultProcessor().EncodeWithConfig(value, cfg)
}

// EncodePretty converts any Go value to pretty-formatted JSON string
func EncodePretty(value any, config ...*EncodeConfig) (string, error) {
	var cfg *EncodeConfig
	if len(config) > 0 && config[0] != nil {
		cfg = config[0]
	} else {
		cfg = NewPrettyConfig()
	}
	return getDefaultProcessor().EncodeWithConfig(value, cfg)
}

// FormatPretty formats JSON string with pretty indentation.
func FormatPretty(jsonStr string, opts ...*ProcessorOptions) (string, error) {
	return getDefaultProcessor().FormatPretty(jsonStr, opts...)
}

// FormatCompact removes whitespace from JSON string.
func FormatCompact(jsonStr string, opts ...*ProcessorOptions) (string, error) {
	return getDefaultProcessor().Compact(jsonStr, opts...)
}

// Print prints any Go value as JSON to stdout in compact format.
// Note: Writes errors to stderr. Use PrintE for error handling.
func Print(data any) {
	result, err := printData(data, false)
	if err != nil {
		fmt.Fprintf(os.Stderr, "json.Print error: %v\n", err)
		return
	}
	fmt.Println(result)
}

// PrintPretty prints any Go value as formatted JSON to stdout.
// Note: Writes errors to stderr. Use PrintPrettyE for error handling.
func PrintPretty(data any) {
	result, err := printData(data, true)
	if err != nil {
		fmt.Fprintf(os.Stderr, "json.PrintPretty error: %v\n", err)
		return
	}
	fmt.Println(result)
}

// PrintE prints any Go value as JSON to stdout in compact format.
// Returns an error instead of writing to stderr, allowing callers to handle errors.
func PrintE(data any) error {
	result, err := printData(data, false)
	if err != nil {
		return fmt.Errorf("json.Print error: %w", err)
	}
	fmt.Println(result)
	return nil
}

// PrintPrettyE prints any Go value as formatted JSON to stdout.
// Returns an error instead of writing to stderr, allowing callers to handle errors.
func PrintPrettyE(data any) error {
	result, err := printData(data, true)
	if err != nil {
		return fmt.Errorf("json.PrintPretty error: %w", err)
	}
	fmt.Println(result)
	return nil
}

// printData handles the core logic for Print and PrintPretty
func printData(data any, pretty bool) (string, error) {
	processor := getDefaultProcessor()

	switch v := data.(type) {
	case string:
		// Check if it's valid JSON - if so, format it directly
		if isValid, _ := processor.Valid(v); isValid {
			if pretty {
				return processor.FormatPretty(v)
			}
			return processor.Compact(v)
		}
		// Not valid JSON, encode as a normal string
		if pretty {
			return EncodePretty(v)
		}
		return Encode(v)

	case []byte:
		jsonStr := string(v)
		// Check if it's valid JSON - if so, format it directly
		if isValid, _ := processor.Valid(jsonStr); isValid {
			if pretty {
				return processor.FormatPretty(jsonStr)
			}
			return processor.Compact(jsonStr)
		}
		// Not valid JSON, encode as normal
		if pretty {
			return EncodePretty(v)
		}
		return Encode(v)

	default:
		// Encode other types normally
		if pretty {
			return EncodePretty(v)
		}
		return Encode(v)
	}
}

// Valid reports whether data is valid JSON
func Valid(data []byte) bool {
	jsonStr := string(data)
	valid, err := getDefaultProcessor().Valid(jsonStr)
	return err == nil && valid
}

// ValidateSchema validates JSON data against a schema
func ValidateSchema(jsonStr string, schema *Schema, opts ...*ProcessorOptions) ([]ValidationError, error) {
	return getDefaultProcessor().ValidateSchema(jsonStr, schema, opts...)
}

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
		result = DefaultOptionsClone() // Use clone since we modify the result
	}
	override(result)
	return result
}
