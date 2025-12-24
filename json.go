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
	stdJSON "encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// Global processor instance for convenience functions with optimized synchronization
var (
	defaultProcessor   *Processor
	defaultProcessorMu sync.RWMutex
)

// getDefaultProcessor returns the global default processor instance with optimized locking
func getDefaultProcessor() *Processor {
	// Fast path: read lock for existing processor
	defaultProcessorMu.RLock()
	if defaultProcessor != nil && !defaultProcessor.IsClosed() {
		p := defaultProcessor
		defaultProcessorMu.RUnlock()
		return p
	}
	defaultProcessorMu.RUnlock()

	// Slow path: write lock for initialization
	defaultProcessorMu.Lock()
	defer defaultProcessorMu.Unlock()

	// Double-check after acquiring write lock
	if defaultProcessor == nil || defaultProcessor.IsClosed() {
		defaultProcessor = New()
	}

	return defaultProcessor
}

// SetGlobalProcessor allows setting a custom global processor (thread-safe)
func SetGlobalProcessor(processor *Processor) {
	if processor == nil {
		return
	}

	defaultProcessorMu.Lock()
	defer defaultProcessorMu.Unlock()

	// Close old processor if it exists
	if defaultProcessor != nil {
		defaultProcessor.Close()
	}

	defaultProcessor = processor
}

// ShutdownGlobalProcessor gracefully shuts down the global processor
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
	return getTypedWithProcessor[T](getDefaultProcessor(), jsonStr, path, opts...)
}

// GetString retrieves a string value from JSON
func GetString(jsonStr, path string, opts ...*ProcessorOptions) (string, error) {
	return GetTyped[string](jsonStr, path, opts...)
}

// GetInt retrieves an int value from JSON
func GetInt(jsonStr, path string, opts ...*ProcessorOptions) (int, error) {
	return GetTyped[int](jsonStr, path, opts...)
}

// GetFloat64 retrieves a float64 value from JSON
func GetFloat64(jsonStr, path string, opts ...*ProcessorOptions) (float64, error) {
	return GetTyped[float64](jsonStr, path, opts...)
}

// GetBool retrieves a bool value from JSON
func GetBool(jsonStr, path string, opts ...*ProcessorOptions) (bool, error) {
	return GetTyped[bool](jsonStr, path, opts...)
}

// GetArray retrieves an array from JSON
func GetArray(jsonStr, path string, opts ...*ProcessorOptions) ([]any, error) {
	return GetTyped[[]any](jsonStr, path, opts...)
}

// GetObject retrieves an object from JSON
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
	// First get the raw value to check if it was null in JSON
	rawValue, rawErr := Get(jsonStr, path, opts...)
	if rawErr != nil || rawValue == nil {
		return defaultValue
	}

	// Now get the typed value
	value, err := GetTyped[T](jsonStr, path, opts...)
	if err != nil {
		return defaultValue
	}

	return value
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
	// Create options with path creation enabled
	createOpts := &ProcessorOptions{
		CreatePaths: true,
	}

	// Merge with provided options
	if len(opts) > 0 && opts[0] != nil {
		createOpts.StrictMode = opts[0].StrictMode
		createOpts.CacheResults = opts[0].CacheResults
		createOpts.MaxDepth = opts[0].MaxDepth
		createOpts.AllowComments = opts[0].AllowComments
		createOpts.PreserveNumbers = opts[0].PreserveNumbers
		createOpts.CleanupNulls = opts[0].CleanupNulls
		createOpts.CompactArrays = opts[0].CompactArrays
		createOpts.ContinueOnError = opts[0].ContinueOnError
		// Keep CreatePaths as true
	}

	return getDefaultProcessor().SetMultiple(jsonStr, updates, createOpts)
}

// Delete deletes a value from JSON at the specified path
func Delete(jsonStr, path string, opts ...*ProcessorOptions) (string, error) {
	return getDefaultProcessor().Delete(jsonStr, path, opts...)
}

// DeleteWithCleanNull removes a value from JSON and cleans up null values
func DeleteWithCleanNull(jsonStr, path string, opts ...*ProcessorOptions) (string, error) {
	// Create options with cleanup enabled
	cleanupOpts := &ProcessorOptions{
		CleanupNulls:  true,
		CompactArrays: true,
	}

	// Merge with provided options
	if len(opts) > 0 && opts[0] != nil {
		cleanupOpts.StrictMode = opts[0].StrictMode
		cleanupOpts.CacheResults = opts[0].CacheResults
		cleanupOpts.MaxDepth = opts[0].MaxDepth
		cleanupOpts.AllowComments = opts[0].AllowComments
		cleanupOpts.PreserveNumbers = opts[0].PreserveNumbers
		cleanupOpts.CreatePaths = opts[0].CreatePaths
		cleanupOpts.ContinueOnError = opts[0].ContinueOnError
		// Keep cleanup settings as true
	}

	processor := getDefaultProcessor()
	return processor.Delete(jsonStr, path, cleanupOpts)
}

// LoadFromFile loads JSON data from a file
func LoadFromFile(filename string) (string, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return "", fmt.Errorf("failed to read file %s: %w", filename, err)
	}
	return string(data), nil
}

// SaveToFile saves JSON data to a file with optional formatting
func SaveToFile(filePath string, data any, pretty ...bool) error {
	proc := getDefaultProcessor()
	if err := proc.validateFilePath(filePath); err != nil {
		return err
	}

	if err := createDirectoryIfNotExists(filePath); err != nil {
		return fmt.Errorf("failed to create directory for %s: %w", filePath, err)
	}

	shouldFormat := len(pretty) > 0 && pretty[0]

	var jsonBytes []byte
	var err error

	switch v := data.(type) {
	case string:
		var parsed any
		if err := stdJSON.Unmarshal([]byte(v), &parsed); err != nil {
			return fmt.Errorf("invalid JSON string: %w", err)
		}
		jsonBytes, err = marshalWithFormat(parsed, shouldFormat)
	case []byte:
		var parsed any
		if err := stdJSON.Unmarshal(v, &parsed); err != nil {
			return fmt.Errorf("invalid JSON bytes: %w", err)
		}
		jsonBytes, err = marshalWithFormat(parsed, shouldFormat)
	default:
		jsonBytes, err = marshalWithFormat(data, shouldFormat)
	}

	if err != nil {
		return fmt.Errorf("failed to marshal data to JSON: %w", err)
	}

	if err := os.WriteFile(filePath, jsonBytes, 0644); err != nil {
		return fmt.Errorf("failed to write file %s: %w", filePath, err)
	}

	return nil
}

func marshalWithFormat(data any, pretty bool) ([]byte, error) {
	if pretty {
		return stdJSON.MarshalIndent(data, "", "  ")
	}
	return stdJSON.Marshal(data)
}

// createDirectoryIfNotExists creates the directory structure for a file path if it doesn't exist
func createDirectoryIfNotExists(filePath string) error {
	dir := filepath.Dir(filePath)
	if dir == "." || dir == "/" {
		return nil // No directory to create
	}

	// Check if directory already exists
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		// Create directory with appropriate permissions
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}
	return nil
}

// FormatPretty formats JSON with indentation
func FormatPretty(jsonStr string, opts ...*ProcessorOptions) (string, error) {
	return getDefaultProcessor().FormatPretty(jsonStr, opts...)
}

// FormatCompact removes whitespace from JSON
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
	if len(config) > 0 {
		cfg = config[0]
	} else {
		cfg = NewPrettyConfig()
	}
	return getDefaultProcessor().EncodeWithConfig(value, cfg)
}

// EncodeCompact converts any Go value to compact JSON string
func EncodeCompact(value any, config ...*EncodeConfig) (string, error) {
	var cfg *EncodeConfig
	if len(config) > 0 {
		cfg = config[0]
	} else {
		cfg = NewCompactConfig()
	}
	return getDefaultProcessor().EncodeWithConfig(value, cfg)
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
func Compact(dst *bytes.Buffer, src []byte) error {
	compacted, err := FormatCompact(string(src))
	if err != nil {
		return err
	}
	_, err = dst.WriteString(compacted)
	return err
}

// Indent appends to dst an indented form of the JSON-encoded src.
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
func HTMLEscape(dst *bytes.Buffer, src []byte) {
	var data any
	if err := Unmarshal(src, &data); err != nil {
		dst.Write(src)
		return
	}

	config := &EncodeConfig{EscapeHTML: true}
	escaped, err := Encode(data, config)
	if err != nil {
		dst.Write(src)
		return
	}

	dst.WriteString(escaped)
}

// MarshalToFile converts data to JSON and saves it to the specified file.
// This is a convenience function that combines Marshal and file writing operations.
//
// Parameters:
//   - path: file path where JSON will be saved (directories are created automatically)
//   - data: any Go value to be marshaled to JSON
//   - pretty: optional parameter - true for formatted JSON, false for compact (default: false)
//
// Returns error if marshaling fails or file cannot be written.
//
// Example:
//
//	user := map[string]any{"name": "John", "age": 30}
//	err := json.MarshalToFile("data/user.json", user, true)
func MarshalToFile(path string, data any, pretty ...bool) error {
	// Marshal data to JSON bytes
	var jsonBytes []byte
	var err error

	shouldFormat := len(pretty) > 0 && pretty[0]
	if shouldFormat {
		jsonBytes, err = MarshalIndent(data, "", "  ")
	} else {
		jsonBytes, err = Marshal(data)
	}

	if err != nil {
		return fmt.Errorf("failed to marshal data to JSON: %w", err)
	}

	// Use the existing SaveToFile functionality through the processor
	proc := getDefaultProcessor()
	if err := proc.validateFilePath(path); err != nil {
		return err
	}

	if err := createDirectoryIfNotExists(path); err != nil {
		return fmt.Errorf("failed to create directory for %s: %w", path, err)
	}

	// Write JSON bytes to file
	if err := os.WriteFile(path, jsonBytes, 0644); err != nil {
		return fmt.Errorf("failed to write file %s: %w", path, err)
	}

	return nil
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

// getTypedWithProcessor is an internal helper for type-safe operations
func getTypedWithProcessor[T any](proc *Processor, jsonStr, path string, opts ...*ProcessorOptions) (T, error) {
	return GetTypedWithProcessor[T](proc, jsonStr, path, opts...)
}
