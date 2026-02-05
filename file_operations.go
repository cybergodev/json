package json

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// LoadFromFile loads JSON data from a file and returns the raw JSON string
// This matches the package-level LoadFromFile signature for API consistency
func (p *Processor) LoadFromFile(filePath string, opts ...*ProcessorOptions) (string, error) {
	if err := p.checkClosed(); err != nil {
		return "", err
	}

	// Validate file path for security
	if err := p.validateFilePath(filePath); err != nil {
		return "", err
	}

	// Read file
	data, err := os.ReadFile(filePath)
	if err != nil {
		return "", &JsonsError{
			Op:      "load_from_file",
			Message: fmt.Sprintf("failed to read file %s", filePath),
			Err:     fmt.Errorf("read file error: %w", err),
		}
	}

	return string(data), nil
}

// LoadFromFileAsData loads JSON data from a file and returns the parsed data structure
// Use this method when you need the parsed JSON object instead of the raw string
func (p *Processor) LoadFromFileAsData(filePath string, opts ...*ProcessorOptions) (any, error) {
	if err := p.checkClosed(); err != nil {
		return nil, err
	}

	// Validate file path for security
	if err := p.validateFilePath(filePath); err != nil {
		return nil, err
	}

	// Read file
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, &JsonsError{
			Op:      "load_from_file_as_data",
			Message: fmt.Sprintf("failed to read file %s", filePath),
			Err:     fmt.Errorf("read file error: %w", err),
		}
	}

	// Parse JSON
	var jsonData any
	err = p.Parse(string(data), &jsonData, opts...)
	return jsonData, err
}

// preprocessDataForEncoding normalizes string/[]byte inputs to parsed data
// to prevent double-encoding issues when saving JSON files.
//
// This ensures that passing a JSON string (e.g., `{"a":1}`) will write
// the JSON object directly, not a JSON-encoded string ("{\"a\":1}").
func (p *Processor) preprocessDataForEncoding(data any) (any, error) {
	switch v := data.(type) {
	case string:
		// Parse JSON string to prevent double-encoding
		var parsed any
		if err := p.Parse(v, &parsed); err != nil {
			return nil, &JsonsError{
				Op:      "preprocess_data",
				Message: "invalid JSON string input",
				Err:     err,
			}
		}
		return parsed, nil
	case []byte:
		// Parse JSON bytes to prevent double-encoding
		var parsed any
		if err := p.Parse(string(v), &parsed); err != nil {
			return nil, &JsonsError{
				Op:      "preprocess_data",
				Message: "invalid JSON byte input",
				Err:     err,
			}
		}
		return parsed, nil
	default:
		// Return other types as-is (will be encoded normally)
		return data, nil
	}
}

// SaveToFile saves data to a JSON file with automatic directory creation
// Parameters:
//   - filePath: file path and name, creates directories if they don't exist
//   - data: JSON data to save (can be a Go value, JSON string, or JSON bytes)
//   - pretty: optional parameter - true for formatted JSON, false for compact JSON (default: false)
//
// Special behavior for string and []byte inputs:
//   - If data is a JSON string, it will be parsed first to prevent double-encoding.
//   - If data is []byte containing JSON, it will be parsed first.
//   - This ensures that SaveToFile("file.json", `{"a":1}`) writes {"a":1} not "{\"a\":1}"
func (p *Processor) SaveToFile(filePath string, data any, pretty ...bool) error {
	if err := p.checkClosed(); err != nil {
		return err
	}

	// Validate file path for security
	if err := p.validateFilePath(filePath); err != nil {
		return err
	}

	// Create directory if it doesn't exist
	if err := p.createDirectoryIfNotExists(filePath); err != nil {
		return &JsonsError{
			Op:      "save_to_file",
			Message: fmt.Sprintf("failed to create directory for %s", filePath),
			Err:     fmt.Errorf("directory creation error: %w", err),
		}
	}

	// Preprocess data to prevent double-encoding of string/[]byte inputs
	processedData, err := p.preprocessDataForEncoding(data)
	if err != nil {
		return err
	}

	// Determine formatting preference
	shouldFormat := false
	if len(pretty) > 0 {
		shouldFormat = pretty[0]
	}

	// Encode data to JSON
	config := DefaultEncodeConfig()
	config.Pretty = shouldFormat
	jsonStr, err := p.EncodeWithConfig(processedData, config)
	if err != nil {
		return err
	}

	// Write to file
	err = os.WriteFile(filePath, []byte(jsonStr), 0644)
	if err != nil {
		return &JsonsError{
			Op:      "save_to_file",
			Message: fmt.Sprintf("failed to write file %s", filePath),
			Err:     fmt.Errorf("write file error: %w", err),
		}
	}

	return nil
}

// LoadFromReader loads JSON data from an io.Reader and returns the raw JSON string
// This matches the LoadFromFile behavior for API consistency
func (p *Processor) LoadFromReader(reader io.Reader, opts ...*ProcessorOptions) (string, error) {
	if err := p.checkClosed(); err != nil {
		return "", err
	}

	// Use LimitReader to prevent excessive memory usage
	limitedReader := io.LimitReader(reader, p.config.MaxJSONSize)

	// Read all data
	data, err := io.ReadAll(limitedReader)
	if err != nil {
		return "", &JsonsError{
			Op:      "load_from_reader",
			Message: "failed to read from reader",
			Err:     fmt.Errorf("reader error: %w", err),
		}
	}

	// Check if we hit the size limit
	if int64(len(data)) >= p.config.MaxJSONSize {
		return "", &JsonsError{
			Op:      "load_from_reader",
			Message: fmt.Sprintf("JSON size exceeds maximum %d bytes", p.config.MaxJSONSize),
			Err:     ErrSizeLimit,
		}
	}

	return string(data), nil
}

// LoadFromReaderAsData loads JSON data from an io.Reader and returns the parsed data structure
// Use this method when you need the parsed JSON object instead of the raw string
func (p *Processor) LoadFromReaderAsData(reader io.Reader, opts ...*ProcessorOptions) (any, error) {
	if err := p.checkClosed(); err != nil {
		return nil, err
	}

	// Use LimitReader to prevent excessive memory usage
	limitedReader := io.LimitReader(reader, p.config.MaxJSONSize)

	// Read all data
	data, err := io.ReadAll(limitedReader)
	if err != nil {
		return nil, &JsonsError{
			Op:      "load_from_reader_as_data",
			Message: "failed to read from reader",
			Err:     fmt.Errorf("reader error: %w", err),
		}
	}

	// Check if we hit the size limit
	if int64(len(data)) >= p.config.MaxJSONSize {
		return nil, &JsonsError{
			Op:      "load_from_reader_as_data",
			Message: fmt.Sprintf("JSON size exceeds maximum %d bytes", p.config.MaxJSONSize),
			Err:     ErrSizeLimit,
		}
	}

	// Parse JSON
	var jsonData any
	err = p.Parse(string(data), &jsonData, opts...)
	return jsonData, err
}

// SaveToWriter saves data to an io.Writer
//
// Special behavior for string and []byte inputs:
//   - If data is a JSON string, it will be parsed first to prevent double-encoding.
//   - If data is []byte containing JSON, it will be parsed first.
func (p *Processor) SaveToWriter(writer io.Writer, data any, pretty bool, opts ...*ProcessorOptions) error {
	if err := p.checkClosed(); err != nil {
		return err
	}

	// Preprocess data to prevent double-encoding of string/[]byte inputs
	processedData, err := p.preprocessDataForEncoding(data)
	if err != nil {
		return err
	}

	// Encode data to JSON
	config := DefaultEncodeConfig()
	config.Pretty = pretty
	jsonStr, err := p.EncodeWithConfig(processedData, config, opts...)
	if err != nil {
		return err
	}

	// Write to writer
	_, err = writer.Write([]byte(jsonStr))
	if err != nil {
		return &JsonsError{
			Op:      "save_to_writer",
			Message: fmt.Sprintf("failed to write to writer: %v", err),
			Err:     ErrOperationFailed,
		}
	}

	return nil
}

// createDirectoryIfNotExists creates the directory structure for a file path if it doesn't exist
func (p *Processor) createDirectoryIfNotExists(filePath string) error {
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

// validateFilePath provides enhanced security validation for file paths
func (p *Processor) validateFilePath(filePath string) error {
	if filePath == "" {
		return newOperationError("validate_file_path", "file path cannot be empty", ErrOperationFailed)
	}

	// SECURITY: Check for null bytes before any processing
	if strings.Contains(filePath, "\x00") {
		return newSecurityError("validate_file_path", "null byte in path")
	}

	// SECURITY: Check for path traversal patterns BEFORE normalization
	// This prevents bypassing via encoded sequences or mixed separators
	if containsPathTraversal(filePath) {
		return newSecurityError("validate_file_path", "path traversal pattern detected")
	}

	// Platform-specific security checks on original path (before normalization)
	if runtime.GOOS == "windows" {
		if err := validateWindowsPath(filePath); err != nil {
			return err
		}
	}

	// Normalize the path after security checks
	cleanPath := filepath.Clean(filePath)

	// Check path length after cleaning
	if len(cleanPath) > MaxPathLength {
		return newOperationError("validate_file_path",
			fmt.Sprintf("path too long: %d > %d", len(cleanPath), MaxPathLength),
			ErrOperationFailed)
	}

	// Convert to absolute path for further validation
	absPath, err := filepath.Abs(cleanPath)
	if err != nil {
		return fmt.Errorf("invalid path: %w", err)
	}

	// Platform-specific security checks on absolute path
	if runtime.GOOS != "windows" {
		if err := validateUnixPath(absPath); err != nil {
			return err
		}
	}

	// Check symlinks with loop detection
	if info, err := os.Lstat(absPath); err == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			realPath, err := filepath.EvalSymlinks(absPath)
			if err != nil {
				return fmt.Errorf("cannot resolve symlink: %w", err)
			}

			// Ensure symlink doesn't escape to restricted areas
			if runtime.GOOS != "windows" {
				if err := validateUnixPath(realPath); err != nil {
					return err
				}
			} else {
				if err := validateWindowsPath(realPath); err != nil {
					return err
				}
			}
		}
	}

	// Check file size if exists
	if info, err := os.Stat(absPath); err == nil {
		if info.Size() > p.config.MaxJSONSize {
			return newSizeLimitError("validate_file_path", info.Size(), p.config.MaxJSONSize)
		}
	}

	return nil
}

// containsPathTraversal checks for path traversal patterns comprehensively
// SECURITY FIX: Enhanced detection with comprehensive bypass patterns
func containsPathTraversal(path string) bool {
	// Check for various path traversal patterns including bypass attempts
	patterns := []string{
		"..",         // Standard traversal
		"%2e%2e",     // URL encoded
		"%252e%252e", // Double URL encoded
		"..%2f",      // Mixed encoding
		"..%5c",      // Windows backslash encoded
		"..%c0%af",   // UTF-8 overlong encoding
		"..%c1%9c",   // UTF-8 overlong encoding variant
		".%2e",       // Partial encoding
		"%2e.",       // Partial encoding variant
		"%2e%2e%2f",  // Full URL encoded traversal
		"%2e%2e%5c",  // Windows variant
		"..\\",       // Windows backslash
		"..\\/",      // Mixed separators
		"..%00",      // Null byte injection
		"..%0a",      // Newline injection
		"..%0d",      // Carriage return injection
		"..%09",      // Tab injection
		"..%20",      // Space injection
		"....//",     // Double dot with double slash
		"....\\\\",   // Double dot with double backslash
		".....",      // Five consecutive dots
		"......",     // Six consecutive dots (defense in depth)
	}

	lowerPath := strings.ToLower(path)
	for _, pattern := range patterns {
		if strings.Contains(lowerPath, pattern) {
			return true
		}
	}

	// Additional check: consecutive dots in any form (3 or more)
	if containsConsecutiveDots(path, 3) {
		return true
	}

	// Check for encoded null bytes and control characters
	encodedNulls := []string{"%00", "%0a", "%0d", "%09", "%20"}
	for _, encoded := range encodedNulls {
		if strings.Contains(lowerPath, encoded) {
			return true
		}
	}

	// Check for partial double encoding bypass attempts (e.g., %2e%2%2e)
	if containsPartialDoubleEncoding(lowerPath) {
		return true
	}

	return false
}

// containsConsecutiveDots checks for consecutive dots in any form
func containsConsecutiveDots(path string, minCount int) bool {
	dotCount := 0
	for _, r := range path {
		if r == '.' {
			dotCount++
			if dotCount >= minCount {
				return true
			}
		} else {
			dotCount = 0
		}
	}
	return false
}

// containsPartialDoubleEncoding checks for partial double encoding bypass attempts
func containsPartialDoubleEncoding(path string) bool {
	// Check for patterns like %2e%2%2e (partial double encoding)
	patterns := []string{"%2e%2", "%25%2e", "%2f%2", "%5c%2"}
	for _, pattern := range patterns {
		if strings.Contains(path, pattern) {
			return true
		}
	}
	return false
}

// validateUnixPath validates Unix-specific path security
func validateUnixPath(absPath string) error {
	lowerPath := strings.ToLower(absPath)

	// Block access to critical system directories
	criticalDirs := []string{
		"/dev/",
		"/proc/",
		"/sys/",
		"/etc/passwd",
		"/etc/shadow",
		"/etc/sudoers",
		"/etc/hosts",
		"/etc/fstab",
		"/etc/crontab",
		"/root/",
		"/boot/",
		"/var/log/",
		"/usr/bin/",
		"/usr/sbin/",
		"/sbin/",
		"/bin/",
	}

	for _, dir := range criticalDirs {
		if strings.HasPrefix(lowerPath, dir) {
			return newSecurityError("validate_unix_path", "access to system directory not allowed")
		}
	}

	// Additional security checks for Unix systems
	if strings.Contains(absPath, "/..") || strings.Contains(absPath, "../") {
		return newSecurityError("validate_unix_path", "path traversal detected")
	}

	return nil
}

// validateWindowsPath validates Windows-specific path security
func validateWindowsPath(absPath string) error {
	// Check for UNC paths
	if strings.HasPrefix(absPath, "\\\\") || strings.HasPrefix(absPath, "//") {
		return newSecurityError("validate_windows_path", "UNC paths not allowed")
	}

	// Extract filename for device name checking
	filename := strings.ToUpper(filepath.Base(absPath))
	if idx := strings.LastIndex(filename, "."); idx > 0 {
		filename = filename[:idx]
	}

	// Check reserved device names (complete list)
	reserved := []string{"CON", "PRN", "AUX", "NUL", "CONIN$", "CONOUT$"}
	for _, name := range reserved {
		if filename == name {
			return newSecurityError("validate_windows_path", "Windows reserved device name")
		}
	}

	// Additional check for alternate data streams (ADS) which could bypass validation
	// ADS pattern: "filename:stream_name" or "filename:$DATA"
	if strings.Contains(absPath, ":") {
		// Split on first colon to check for ADS pattern
		parts := strings.SplitN(absPath, ":", 2)
		if len(parts) == 2 {
			// Check if it looks like a drive letter pattern (e.g., "C:")
			// Drive letter pattern: single letter followed by colon at position 1
			if len(parts[0]) == 1 && parts[0][0] >= 'A' && parts[0][0] <= 'Z' {
				// This is a drive letter path, not ADS
			} else {
				// This is an ADS pattern
				return newSecurityError("validate_windows_path", "alternate data streams not allowed")
			}
		}
	}

	// Check COM1-9 and LPT1-9 (COM0/LPT0 are NOT valid in Windows)
	if len(filename) == 4 && filename[3] >= '0' && filename[3] <= '9' {
		prefix := filename[:3]
		if prefix == "COM" || prefix == "LPT" {
			return newSecurityError("validate_windows_path", "Windows reserved device name")
		}
	}

	// Check COM0 and LPT0 (explicitly invalid in Windows)
	if filename == "COM0" || filename == "LPT0" {
		return newSecurityError("validate_windows_path", "Windows reserved device name")
	}

	// Check for invalid characters in Windows paths (excluding drive letter colon)
	pathToCheck := absPath
	if len(absPath) > 2 && absPath[1] == ':' {
		pathToCheck = absPath[2:]
	}

	invalidChars := []string{"<", ">", ":", "\"", "|", "?", "*"}
	for _, char := range invalidChars {
		if strings.Contains(pathToCheck, char) {
			return newSecurityError("validate_windows_path", "invalid character in path")
		}
	}

	return nil
}

// MarshalToFile converts data to JSON and saves it to the specified file.
// This method provides the same functionality as the package-level MarshalToFile but with processor-specific configuration.
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
func (p *Processor) MarshalToFile(path string, data any, pretty ...bool) error {
	if err := p.checkClosed(); err != nil {
		return err
	}

	// Validate file path for security
	if err := p.validateFilePath(path); err != nil {
		return err
	}

	// Create directory if it doesn't exist
	if err := p.createDirectoryIfNotExists(path); err != nil {
		return fmt.Errorf("failed to create directory for %s: %w", path, err)
	}

	// Preprocess data to prevent double-encoding of string/[]byte inputs
	processedData, err := p.preprocessDataForEncoding(data)
	if err != nil {
		return err
	}

	// Marshal data to JSON bytes
	var jsonBytes []byte

	shouldFormat := len(pretty) > 0 && pretty[0]
	if shouldFormat {
		jsonBytes, err = p.MarshalIndent(processedData, "", "  ")
	} else {
		jsonBytes, err = p.Marshal(processedData)
	}

	if err != nil {
		return fmt.Errorf("failed to marshal data to JSON: %w", err)
	}

	// Write JSON bytes to file
	if err := os.WriteFile(path, jsonBytes, 0644); err != nil {
		return fmt.Errorf("failed to write file %s: %w", path, err)
	}

	return nil
}

// UnmarshalFromFile reads JSON data from the specified file and unmarshals it into the provided value.
// This method provides the same functionality as the package-level UnmarshalFromFile but with processor-specific configuration.
//
// Parameters:
//   - path: file path to read JSON from
//   - v: pointer to the value where JSON will be unmarshaled
//   - opts: optional processor options for unmarshaling
//
// Returns error if file cannot be read or JSON unmarshaling fails.
func (p *Processor) UnmarshalFromFile(path string, v any, opts ...*ProcessorOptions) error {
	if err := p.checkClosed(); err != nil {
		return err
	}

	// Validate input parameters
	if v == nil {
		return fmt.Errorf("unmarshal target cannot be nil")
	}

	// Validate file path for security
	if err := p.validateFilePath(path); err != nil {
		return err
	}

	// Read file contents with size validation
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read file %s: %w", path, err)
	}

	// Check file size against processor limits
	if int64(len(data)) > p.config.MaxJSONSize {
		return fmt.Errorf("file size %d exceeds maximum allowed size %d", len(data), p.config.MaxJSONSize)
	}

	// Unmarshal JSON data using processor's Unmarshal method
	if err := p.Unmarshal(data, v, opts...); err != nil {
		return fmt.Errorf("failed to unmarshal JSON from file %s: %w", path, err)
	}

	return nil
}
