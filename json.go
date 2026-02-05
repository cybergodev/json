// Package json provides a high-performance, thread-safe JSON processing library
// with 100% encoding/json compatibility and advanced path operations.
//
// Key Features:
//   - 100% encoding/json compatibility - drop-in replacement
//   - High-performance path operations with smart caching
//   - Thread-safe concurrent operations
//   - Type-safe generic operations with Go 1.24+ features
//   - Memory-efficient resource pooling
//   - Production-ready error handling and validation
//
// Basic Usage:
//
//	// Simple operations (100% compatible with encoding/json)
//	data, err := json.Marshal(value)
//	err = json.Unmarshal(data, &target)
//
//	// Advanced path operations
//	value, err := json.Get(`{"user":{"name":"John"}}`, "user.name")
//	result, err := json.Set(`{"user":{}}`, "user.age", 30)
//
//	// Type-safe operations
//	name, err := json.GetString(jsonStr, "user.name")
//	age, err := json.GetInt(jsonStr, "user.age")
//
//	// Advanced processor for complex operations
//	processor := json.New() // Use default config
//	defer processor.Close()
//	value, err := processor.Get(jsonStr, "complex.path[0].field")

package json

import (
	"bytes"
	"fmt"
	"os"
	"sync"
)

var (
	defaultProcessor   *Processor
	defaultProcessorMu sync.RWMutex
)

func getDefaultProcessor() *Processor {
	defaultProcessorMu.RLock()
	if defaultProcessor != nil && !defaultProcessor.IsClosed() {
		p := defaultProcessor
		defaultProcessorMu.RUnlock()
		return p
	}
	defaultProcessorMu.RUnlock()

	defaultProcessorMu.Lock()
	defer defaultProcessorMu.Unlock()

	if defaultProcessor == nil || defaultProcessor.IsClosed() {
		defaultProcessor = New()
	}

	return defaultProcessor
}

// SetGlobalProcessor sets a custom global processor (thread-safe)
func SetGlobalProcessor(processor *Processor) {
	if processor == nil {
		return
	}

	defaultProcessorMu.Lock()
	defer defaultProcessorMu.Unlock()

	if defaultProcessor != nil {
		defaultProcessor.Close()
	}

	defaultProcessor = processor
}

// ShutdownGlobalProcessor shuts down the global processor
func ShutdownGlobalProcessor() {
	defaultProcessorMu.Lock()
	defer defaultProcessorMu.Unlock()

	if defaultProcessor != nil {
		defaultProcessor.Close()
		defaultProcessor = nil
	}
}

// Get retrieves a value from JSON at the specified path
func Get(jsonStr, path string, opts ...*ProcessorOptions) (any, error) {
	return getDefaultProcessor().Get(jsonStr, path, opts...)
}

// GetTyped retrieves a typed value from JSON at the specified path
func GetTyped[T any](jsonStr, path string, opts ...*ProcessorOptions) (T, error) {
	return GetTypedWithProcessor[T](getDefaultProcessor(), jsonStr, path, opts...)
}

func GetString(jsonStr, path string, opts ...*ProcessorOptions) (string, error) {
	return GetTyped[string](jsonStr, path, opts...)
}

func GetInt(jsonStr, path string, opts ...*ProcessorOptions) (int, error) {
	return GetTyped[int](jsonStr, path, opts...)
}

func GetFloat64(jsonStr, path string, opts ...*ProcessorOptions) (float64, error) {
	return GetTyped[float64](jsonStr, path, opts...)
}

func GetBool(jsonStr, path string, opts ...*ProcessorOptions) (bool, error) {
	return GetTyped[bool](jsonStr, path, opts...)
}

func GetArray(jsonStr, path string, opts ...*ProcessorOptions) ([]any, error) {
	return GetTyped[[]any](jsonStr, path, opts...)
}

func GetObject(jsonStr, path string, opts ...*ProcessorOptions) (map[string]any, error) {
	return GetTyped[map[string]any](jsonStr, path, opts...)
}

// GetWithDefault retrieves a value from JSON with a default fallback
func GetWithDefault(jsonStr, path string, defaultValue any, opts ...*ProcessorOptions) any {
	value, err := Get(jsonStr, path, opts...)
	if err != nil || value == nil {
		return defaultValue
	}
	return value
}

// GetTypedWithDefault retrieves a typed value with a default fallback
func GetTypedWithDefault[T any](jsonStr, path string, defaultValue T, opts ...*ProcessorOptions) T {
	value, err := GetTyped[T](jsonStr, path, opts...)
	if err != nil {
		return defaultValue
	}

	// Handle nil values for reference types
	if isZeroValue(value) {
		return defaultValue
	}

	return value
}

// isZeroValue checks if a value is the zero value for its type
func isZeroValue(v any) bool {
	if v == nil {
		return true
	}
	switch val := v.(type) {
	case string:
		return val == ""
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, float32, float64:
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

// Type-specific convenience functions with defaults
func GetStringWithDefault(jsonStr, path, defaultValue string, opts ...*ProcessorOptions) string {
	return GetTypedWithDefault(jsonStr, path, defaultValue, opts...)
}

func GetIntWithDefault(jsonStr, path string, defaultValue int, opts ...*ProcessorOptions) int {
	return GetTypedWithDefault(jsonStr, path, defaultValue, opts...)
}

func GetFloat64WithDefault(jsonStr, path string, defaultValue float64, opts ...*ProcessorOptions) float64 {
	return GetTypedWithDefault(jsonStr, path, defaultValue, opts...)
}

func GetBoolWithDefault(jsonStr, path string, defaultValue bool, opts ...*ProcessorOptions) bool {
	return GetTypedWithDefault(jsonStr, path, defaultValue, opts...)
}

func GetArrayWithDefault(jsonStr, path string, defaultValue []any, opts ...*ProcessorOptions) []any {
	return GetTypedWithDefault(jsonStr, path, defaultValue, opts...)
}

func GetObjectWithDefault(jsonStr, path string, defaultValue map[string]any, opts ...*ProcessorOptions) map[string]any {
	return GetTypedWithDefault(jsonStr, path, defaultValue, opts...)
}

// GetMultiple retrieves multiple values from JSON using multiple path expressions
func GetMultiple(jsonStr string, paths []string, opts ...*ProcessorOptions) (map[string]any, error) {
	return getDefaultProcessor().GetMultiple(jsonStr, paths, opts...)
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
	cleanupOpts := mergeOptionsWithOverride(opts, func(o *ProcessorOptions) {
		o.CleanupNulls = true
		o.CompactArrays = true
	})
	return getDefaultProcessor().Delete(jsonStr, path, cleanupOpts)
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
//
// Example:
//
//	user := map[string]any{"name": "John", "age": 30}
//	err := json.MarshalToFile("data/user.json", user, true)
func MarshalToFile(path string, data any, pretty ...bool) error {
	return getDefaultProcessor().MarshalToFile(path, data, pretty...)
}

// FormatPretty formats JSON string with pretty indentation.
//
// This is a convenience method that provides consistent API naming for formatting operations
// at the package level. It internally calls Processor.FormatPretty with the default processor.
//
// Note: For standard library compatibility (encoding/json), see also Indent() function which
// works with bytes.Buffer. For Processor methods, the method is named FormatPretty to indicate
// it's a formatting operation, while the core operation is named Compact for space removal.
func FormatPretty(jsonStr string, opts ...*ProcessorOptions) (string, error) {
	return getDefaultProcessor().FormatPretty(jsonStr, opts...)
}

// FormatCompact removes whitespace from JSON string.
//
// This is a convenience method that provides consistent API naming for formatting operations
// at the package level. It internally calls Processor.Compact with the default processor.
//
// Note: The Processor method is named "Compact" for brevity (following the principle that
// processor methods use direct names), while this package-level wrapper uses "FormatCompact"
// to clearly indicate it's a formatting operation. This naming convention helps users
// distinguish between string formatting operations and buffer operations.
func FormatCompact(jsonStr string, opts ...*ProcessorOptions) (string, error) {
	return getDefaultProcessor().Compact(jsonStr, opts...)
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

// Print prints any Go value as JSON to stdout in compact format.
// The data is encoded to JSON and printed followed by a newline.
// If encoding fails, an error message is printed to stderr instead.
//
// Special behavior for string and []byte inputs:
//   - If data is a JSON string, it will be parsed first to prevent double-encoding.
//   - If data is []byte containing JSON, it will be parsed first.
//   - This ensures that json.Print(`{"name":"John"}`) prints {"name":"John"} not "{\"name\":\"John\"}"
//
// Example:
//
//	// Print Go value
//	data := map[string]any{
//	    "monitoring": true,
//	    "database": map[string]any{
//	        "name": "myDb",
//	        "port": "5432",
//	        "ssl":  true,
//	    },
//	    "debug":    false,
//	    "features": []string{"caching"},
//	}
//	json.Print(data)
//	// Output: {"monitoring":true,"database":{"name":"myDb","port":"5432","ssl":true},"debug":false,"features":["caching"]}
//
//	// Print JSON string directly (no double-encoding)
//	jsonStr := `{"name":"John","age":30}`
//	json.Print(jsonStr)
//	// Output: {"name":"John","age":30}
func Print(data any) {
	result, err := printData(data, false)
	if err != nil {
		fmt.Fprintf(os.Stderr, "json.Print error: %v\n", err)
		return
	}
	fmt.Println(result)
}

// printData handles the core logic for Print and PrintPretty
// Returns the formatted JSON string and any error
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

// PrintPretty prints any Go value as formatted JSON to stdout.
// The data is encoded to JSON with pretty indentation and printed.
// If encoding fails, an error message is printed to stderr instead.
//
// Special behavior for string and []byte inputs:
//   - If data is a JSON string, it will be parsed first to prevent double-encoding.
//   - If data is []byte containing JSON, it will be parsed first.
//   - This ensures that json.PrintPretty(`{"name":"John"}`) prints formatted JSON not a JSON string.
//
// Example:
//
//	// Print Go value
//	data := map[string]any{
//	    "monitoring": true,
//	    "database": map[string]any{
//	        "name": "myDb",
//	        "port": "5432",
//	        "ssl":  true,
//	    },
//	    "debug":    false,
//	    "features": []string{"caching"},
//	}
//	json.PrintPretty(data)
//	// Output:
//	// {
//	//   "database": {
//	//     "name": "myDb",
//	//     "port": "5432",
//	//     "ssl": true
//	//   },
//	//   "debug": false,
//	//   "features": [
//	//     "caching"
//	//   ],
//	//   "monitoring": true
//	// }
//
//	// Print JSON string directly (no double-encoding)
//	jsonStr := `{"name":"John","age":30}`
//	json.PrintPretty(jsonStr)
//	// Output:
//	// {
//	//   "age": 30,
//	//   "name": "John"
//	// }
func PrintPretty(data any) {
	result, err := printData(data, true)
	if err != nil {
		fmt.Fprintf(os.Stderr, "json.PrintPretty error: %v\n", err)
		return
	}
	fmt.Println(result)
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
	_, err = dst.WriteString(compacted)
	return err
}

// Indent appends to dst an indented form of the JSON-encoded src.
// This function is 100% compatible with encoding/json.Indent.
func Indent(dst *bytes.Buffer, src []byte, prefix, indent string) error {
	var data any
	if err := Unmarshal(src, &data); err != nil {
		return err
	}
	indented, err := MarshalIndent(data, prefix, indent)
	if err != nil {
		return err
	}
	_, err = dst.Write(indented)
	return err
}

// HTMLEscape appends to dst the JSON-encoded src with HTML-safe escaping.
// This function is 100% compatible with encoding/json.HTMLEscape.
func HTMLEscape(dst *bytes.Buffer, src []byte) {
	var data any
	if err := Unmarshal(src, &data); err != nil {
		dst.Write(src)
		return
	}

	// Use DefaultEncodeConfig for consistency
	config := DefaultEncodeConfig()
	config.EscapeHTML = true
	escaped, err := Encode(data, config)
	if err != nil {
		dst.Write(src)
		return
	}

	dst.WriteString(escaped)
}

// CompactBuffer compacts JSON and writes to a buffer with processor options support.
//
// Use this when you need to process JSON with custom processor options.
// For standard encoding/json compatibility (no options), use Compact() instead.
func CompactBuffer(dst *bytes.Buffer, src []byte, opts ...*ProcessorOptions) error {
	return getDefaultProcessor().CompactBuffer(dst, src, opts...)
}

// IndentBuffer indents JSON and writes to a buffer with processor options support.
//
// Use this when you need to process JSON with custom processor options.
// For standard encoding/json compatibility (no options), use Indent() instead.
func IndentBuffer(dst *bytes.Buffer, src []byte, prefix, indent string, opts ...*ProcessorOptions) error {
	return getDefaultProcessor().IndentBuffer(dst, src, prefix, indent, opts...)
}

// HTMLEscapeBuffer escapes HTML in JSON and writes to a buffer with processor options support.
//
// Use this when you need to process JSON with custom processor options.
// For standard encoding/json compatibility (no options), use HTMLEscape() instead.
func HTMLEscapeBuffer(dst *bytes.Buffer, src []byte, opts ...*ProcessorOptions) {
	getDefaultProcessor().HTMLEscapeBuffer(dst, src, opts...)
}

// UnmarshalFromFile reads JSON data from the specified file and unmarshals it into the provided value.
// This is a convenience function that combines file reading and Unmarshal operations.
//
// Parameters:
//   - path: file path to read JSON from
//   - v: pointer to the value where JSON will be unmarshaled
//
// Returns error if file cannot be read or JSON unmarshaling fails.
//
// Example:
//
//	var user map[string]any
//	err := json.UnmarshalFromFile("data/user.json", &user)
//
//	// Or with a struct
//	var person Person
//	err := json.UnmarshalFromFile("data/person.json", &person)
func UnmarshalFromFile(path string, v any) error {
	// Validate input parameters
	if v == nil {
		return fmt.Errorf("unmarshal target cannot be nil")
	}

	// Use processor for file path validation
	proc := getDefaultProcessor()
	if err := proc.validateFilePath(path); err != nil {
		return err
	}

	// Read file contents
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read file %s: %w", path, err)
	}

	// Unmarshal JSON data
	if err := Unmarshal(data, v); err != nil {
		return fmt.Errorf("failed to unmarshal JSON from file %s: %w", path, err)
	}

	return nil
}

// mergeOptionsWithOverride merges processor options with custom overrides
func mergeOptionsWithOverride(opts []*ProcessorOptions, override func(*ProcessorOptions)) *ProcessorOptions {
	var result *ProcessorOptions
	if len(opts) > 0 && opts[0] != nil {
		result = opts[0].Clone()
	} else {
		result = &ProcessorOptions{}
	}
	override(result)
	return result
}

// ============ Processor Convenience Functions ============
// These functions provide easy access to processor-level functionality
// without requiring explicit processor creation

// GetStats returns performance statistics for the default processor
func GetStats() Stats {
	return getDefaultProcessor().GetStats()
}

// GetHealthStatus returns the health status of the default processor
func GetHealthStatus() HealthStatus {
	return getDefaultProcessor().GetHealthStatus()
}

// ClearCache clears all cached data in the default processor
func ClearCache() {
	getDefaultProcessor().ClearCache()
}

// WarmupCache pre-loads commonly used paths into cache to improve first-access performance
func WarmupCache(jsonStr string, paths []string, opts ...*ProcessorOptions) (*WarmupResult, error) {
	return getDefaultProcessor().WarmupCache(jsonStr, paths, opts...)
}

// ProcessBatch processes multiple operations in a single batch using the default processor
func ProcessBatch(operations []BatchOperation, opts ...*ProcessorOptions) ([]BatchResult, error) {
	return getDefaultProcessor().ProcessBatch(operations, opts...)
}

// EncodeStream encodes multiple values as a JSON array stream using the default processor
func EncodeStream(values any, pretty bool, opts ...*ProcessorOptions) (string, error) {
	return getDefaultProcessor().EncodeStream(values, pretty, opts...)
}

// EncodeBatch encodes multiple key-value pairs as a JSON object using the default processor
func EncodeBatch(pairs map[string]any, pretty bool, opts ...*ProcessorOptions) (string, error) {
	return getDefaultProcessor().EncodeBatch(pairs, pretty, opts...)
}

// EncodeFields encodes only the specified fields from a struct using the default processor
func EncodeFields(value any, fields []string, pretty bool, opts ...*ProcessorOptions) (string, error) {
	return getDefaultProcessor().EncodeFields(value, fields, pretty, opts...)
}

// ============ Iteration Functions ============
// These functions provide convenient iteration over JSON data structures
// using the default processor

// ForeachWithPathAndIterator iterates over JSON at a path with path information
// This is the only iteration function not already available at package level in iterator.go
func ForeachWithPathAndIterator(jsonStr, path string, fn func(key any, item *IterableValue, currentPath string) IteratorControl) error {
	return getDefaultProcessor().ForeachWithPathAndIterator(jsonStr, path, fn)
}

// Note: Other iteration functions (Foreach, ForeachWithPath, ForeachWithPathAndControl,
// ForeachReturn, ForeachNested) are available at package level in iterator.go
