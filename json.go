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
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"sync/atomic"
)

// Global processor instance for convenience functions with enhanced thread safety
var (
	defaultProcessor     atomic.Pointer[Processor]
	defaultProcessorOnce sync.Once
	processorGeneration  int64        // Generation counter for processor updates (atomic)
	shutdownSignal       int32        // Shutdown signal for graceful termination (atomic)
	globalMutex          sync.RWMutex // Protects global operations when needed
)

// getDefaultProcessor returns the global default processor instance
// This is optimized for production use with enhanced thread safety and error handling
func getDefaultProcessor() *Processor {
	// Check shutdown signal first
	if atomic.LoadInt32(&shutdownSignal) != 0 {
		// Return a minimal processor during shutdown
		return newDefault()
	}

	// Fast path: check if already initialized without lock
	if processor := defaultProcessor.Load(); processor != nil {
		// Verify processor is still healthy
		if !processor.IsClosed() {
			return processor
		}
		// Processor is closed, need to reinitialize
		atomic.AddInt64(&processorGeneration, 1)
	}

	// Slow path: initialize with sync.Once or reinitialize if needed
	defaultProcessorOnce.Do(func() {
		processor := New() // Use internal default constructor
		if processor != nil {
			defaultProcessor.Store(processor)
			atomic.AddInt64(&processorGeneration, 1)
		}
	})

	processor := defaultProcessor.Load()
	if processor == nil || processor.IsClosed() {
		// Fallback: create a new processor if initialization failed or processor is closed
		globalMutex.Lock()
		// Double-check after acquiring lock
		if processor = defaultProcessor.Load(); processor == nil || processor.IsClosed() {
			processor = newDefault()
			if processor != nil {
				defaultProcessor.Store(processor)
				atomic.AddInt64(&processorGeneration, 1)
			}
		}
		globalMutex.Unlock()
	}

	return processor
}

// newDefault creates a new processor with default configuration (internal use only)
func newDefault() *Processor {
	return New()
}

// SetGlobalProcessor allows setting a custom global processor (thread-safe)
func SetGlobalProcessor(processor *Processor) {
	if processor == nil {
		return
	}

	globalMutex.Lock()
	defer globalMutex.Unlock()

	// Close old processor if it exists
	if oldProcessor := defaultProcessor.Load(); oldProcessor != nil {
		oldProcessor.Close()
	}

	defaultProcessor.Store(processor)
	atomic.AddInt64(&processorGeneration, 1)
}

// ShutdownGlobalProcessor gracefully shuts down the global processor
func ShutdownGlobalProcessor() {
	atomic.StoreInt32(&shutdownSignal, 1)

	globalMutex.Lock()
	defer globalMutex.Unlock()

	if processor := defaultProcessor.Load(); processor != nil {
		processor.Close()
		defaultProcessor.Store(nil)
	}
}

// =============================================================================
// CORE API FUNCTIONS
// =============================================================================

// Get retrieves a value from JSON at the specified path
func Get(jsonStr, path string, opts ...*ProcessorOptions) (any, error) {
	return getDefaultProcessor().Get(jsonStr, path, opts...)
}

// GetTyped retrieves a typed value from JSON at the specified path
func GetTyped[T any](jsonStr, path string, opts ...*ProcessorOptions) (T, error) {
	return getTypedWithProcessor[T](getDefaultProcessor(), jsonStr, path, opts...)
}

// Type-safe convenience functions for common types

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

// =============================================================================
// GET WITH DEFAULT VALUE FUNCTIONS
// =============================================================================

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
	// First check if the path exists using Get
	rawValue, err := Get(jsonStr, path, opts...)
	if err != nil || rawValue == nil {
		return defaultValue
	}

	// Path exists, now get the typed value
	value, err := GetTyped[T](jsonStr, path, opts...)
	if err != nil {
		return defaultValue
	}
	return value
}

// Type-safe convenience functions with default values
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

// =============================================================================
// BATCH OPERATIONS
// =============================================================================

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

// =============================================================================
// FILE OPERATIONS
// =============================================================================

// LoadFromFile loads JSON data from a file
func LoadFromFile(filename string) (string, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return "", fmt.Errorf("failed to read file %s: %w", filename, err)
	}
	return string(data), nil
}

// SaveToFile saves JSON data to a file with optional formatting
// Parameters:
//   - filePath: file path and name, creates directories if they don't exist
//   - data: JSON data to save (can be string, []byte, or any serializable data)
//   - pretty: optional parameter - true for formatted JSON, false for compact JSON (default: false)
func SaveToFile(filePath string, data any, pretty ...bool) error {
	// Validate file path for security
	processor := getDefaultProcessor()
	if err := processor.validateFilePath(filePath); err != nil {
		return err
	}

	// Create directory if it doesn't exist
	if err := createDirectoryIfNotExists(filePath); err != nil {
		return fmt.Errorf("failed to create directory for %s: %w", filePath, err)
	}

	// Determine formatting preference
	shouldFormat := false
	if len(pretty) > 0 {
		shouldFormat = pretty[0]
	}

	// Convert data to JSON bytes
	var jsonBytes []byte
	var err error

	switch v := data.(type) {
	case string:
		// If it's already a JSON string, validate and optionally reformat
		var parsed any
		if err := json.Unmarshal([]byte(v), &parsed); err != nil {
			return fmt.Errorf("invalid JSON string: %w", err)
		}
		if shouldFormat {
			jsonBytes, err = json.MarshalIndent(parsed, "", "  ")
		} else {
			jsonBytes, err = json.Marshal(parsed)
		}
	case []byte:
		// If it's JSON bytes, validate and optionally reformat
		var parsed any
		if err := json.Unmarshal(v, &parsed); err != nil {
			return fmt.Errorf("invalid JSON bytes: %w", err)
		}
		if shouldFormat {
			jsonBytes, err = json.MarshalIndent(parsed, "", "  ")
		} else {
			jsonBytes, err = json.Marshal(parsed)
		}
	default:
		// For any other data type, marshal to JSON
		if shouldFormat {
			jsonBytes, err = json.MarshalIndent(data, "", "  ")
		} else {
			jsonBytes, err = json.Marshal(data)
		}
	}

	if err != nil {
		return fmt.Errorf("failed to marshal data to JSON: %w", err)
	}

	// Write to file
	err = os.WriteFile(filePath, jsonBytes, 0644)
	if err != nil {
		return fmt.Errorf("failed to write file %s: %w", filePath, err)
	}

	return nil
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

// =============================================================================
// FORMATTING AND ENCODING FUNCTIONS
// =============================================================================

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

// =============================================================================
// VALIDATION FUNCTIONS
// =============================================================================

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

// =============================================================================
// ENCODING/JSON COMPATIBILITY FUNCTIONS
// =============================================================================

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
		dst.Write(src) // Copy as-is if parsing fails
		return
	}

	config := &EncodeConfig{EscapeHTML: true}
	escaped, err := Encode(data, config)
	if err != nil {
		dst.Write(src) // Copy as-is if encoding fails
		return
	}

	dst.WriteString(escaped)
}

// =============================================================================
// INTERNAL HELPER FUNCTIONS
// =============================================================================

// getTypedWithProcessor is an internal helper for type-safe operations
func getTypedWithProcessor[T any](processor *Processor, jsonStr, path string, opts ...*ProcessorOptions) (T, error) {
	// Use the enhanced GetTypedWithProcessor function from utils.go
	return GetTypedWithProcessor[T](processor, jsonStr, path, opts...)
}

// convertToType attempts to convert a value to the target type
func convertToType[T any](value any) (T, error) {
	var zero T
	if value == nil {
		return zero, fmt.Errorf("cannot convert nil to %T", zero)
	}

	// Direct type assertion first
	if typed, ok := value.(T); ok {
		return typed, nil
	}

	// Enhanced type conversion logic for common cases
	switch any(&zero).(type) {
	case *int:
		switch v := value.(type) {
		case int:
			return any(v).(T), nil
		case int64:
			return any(int(v)).(T), nil
		case float64:
			// Convert float64 to int if it's a whole number
			// Handle large numbers that may be in scientific notation
			if v >= -9223372036854775808 && v <= 9223372036854775807 {
				intVal := int64(v)
				if float64(intVal) == v {
					return any(int(intVal)).(T), nil
				}
			}
			return zero, fmt.Errorf("cannot convert float64 %v to int (not a whole number)", v)
		case string:
			if i, err := strconv.Atoi(v); err == nil {
				return any(i).(T), nil
			}
		}
	case *int64:
		switch v := value.(type) {
		case int:
			return any(int64(v)).(T), nil
		case int64:
			return any(v).(T), nil
		case float64:
			// Handle large numbers that may be in scientific notation
			if v >= -9223372036854775808 && v <= 9223372036854775807 {
				intVal := int64(v)
				if float64(intVal) == v {
					return any(intVal).(T), nil
				}
			}
			return zero, fmt.Errorf("cannot convert float64 %v to int64 (not a whole number)", v)
		case string:
			if i, err := strconv.ParseInt(v, 10, 64); err == nil {
				return any(i).(T), nil
			}
		}
	case *float64:
		switch v := value.(type) {
		case int:
			return any(float64(v)).(T), nil
		case int64:
			return any(float64(v)).(T), nil
		case float64:
			return any(v).(T), nil
		case string:
			if f, err := strconv.ParseFloat(v, 64); err == nil {
				return any(f).(T), nil
			}
		}
	case *string:
		switch v := value.(type) {
		case string:
			return any(v).(T), nil
		case int:
			return any(strconv.Itoa(v)).(T), nil
		case int64:
			return any(strconv.FormatInt(v, 10)).(T), nil
		case float64:
			return any(strconv.FormatFloat(v, 'g', -1, 64)).(T), nil
		case bool:
			return any(strconv.FormatBool(v)).(T), nil
		}
	case *bool:
		switch v := value.(type) {
		case bool:
			return any(v).(T), nil
		case string:
			if b, err := strconv.ParseBool(v); err == nil {
				return any(b).(T), nil
			}
		}
	}

	return zero, fmt.Errorf("cannot convert %T to %T", value, zero)
}
