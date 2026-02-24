package json

import (
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/cybergodev/json/internal"
	"golang.org/x/text/unicode/norm"
)

// LoadFromFile loads JSON data from a file and returns the raw JSON string.
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

// LoadFromFileAsData loads JSON data from a file and returns the parsed data structure.
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

// LoadFromReader loads JSON data from an io.Reader and returns the raw JSON string.
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

// LoadFromReaderAsData loads JSON data from an io.Reader and returns the parsed data structure.
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

// preprocessDataForEncoding normalizes string/[]byte inputs to prevent double-encoding.
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

// createDirectoryIfNotExists creates the directory structure for a file path if needed.
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

// SaveToFile saves data to a JSON file with automatic directory creation
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

// SaveToWriter saves data to an io.Writer
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

// MarshalToFile converts data to JSON and saves it to the specified file.
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
		return &JsonsError{
			Op:      "marshal_to_file",
			Message: fmt.Sprintf("failed to create directory for %s", path),
			Err:     err,
		}
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
		return &JsonsError{
			Op:      "marshal_to_file",
			Message: "failed to marshal data to JSON",
			Err:     err,
		}
	}

	// Write JSON bytes to file
	if err := os.WriteFile(path, jsonBytes, 0644); err != nil {
		return &JsonsError{
			Op:      "marshal_to_file",
			Path:    path,
			Message: fmt.Sprintf("failed to write file %s", path),
			Err:     err,
		}
	}

	return nil
}

// UnmarshalFromFile reads JSON data from the specified file and unmarshals it into the provided value.
func (p *Processor) UnmarshalFromFile(path string, v any, opts ...*ProcessorOptions) error {
	if err := p.checkClosed(); err != nil {
		return err
	}

	// Validate input parameters
	if v == nil {
		return &JsonsError{
			Op:      "unmarshal_from_file",
			Message: "unmarshal target cannot be nil",
			Err:     ErrOperationFailed,
		}
	}

	// Validate file path for security
	if err := p.validateFilePath(path); err != nil {
		return err
	}

	// Read file contents with size validation
	data, err := os.ReadFile(path)
	if err != nil {
		return &JsonsError{
			Op:      "unmarshal_from_file",
			Path:    path,
			Message: fmt.Sprintf("failed to read file %s", path),
			Err:     err,
		}
	}

	// Check file size against processor limits
	if int64(len(data)) > p.config.MaxJSONSize {
		return &JsonsError{
			Op:      "unmarshal_from_file",
			Path:    path,
			Message: fmt.Sprintf("file size %d exceeds maximum allowed size %d", len(data), p.config.MaxJSONSize),
			Err:     ErrSizeLimit,
		}
	}

	// Unmarshal JSON data using processor's Unmarshal method
	if err := p.Unmarshal(data, v, opts...); err != nil {
		return &JsonsError{
			Op:      "unmarshal_from_file",
			Path:    path,
			Message: fmt.Sprintf("failed to unmarshal JSON from file %s", path),
			Err:     err,
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
		return newOperationError("validate_file_path", "invalid path", err)
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
				return newOperationError("validate_file_path", "cannot resolve symlink", err)
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
// Uses case-insensitive matching without allocation for better performance
// Includes Unicode normalization and recursive URL decoding for enhanced security
func containsPathTraversal(path string) bool {
	// SECURITY: First apply Unicode NFC normalization to detect homograph attacks
	// This ensures that visually similar characters are normalized before checking
	normalized := norm.NFC.String(path)

	// SECURITY: Recursively decode URL encoding to catch multi-layered obfuscation
	// Decode up to 3 levels to handle double/triple encoding
	decoded := recursiveURLDecode(normalized)

	// Fast path: check for standard traversal first (most common case)
	if strings.Contains(decoded, "..") {
		return true
	}

	// Also check original path for patterns that might be hidden by encoding
	if strings.Contains(path, "..") {
		return true
	}

	// Check for various path traversal patterns including bypass attempts
	// Using case-insensitive matching without allocation
	patterns := []string{
		"%2e%2e",         // URL encoded
		"%252e%252e",     // Double URL encoded
		"%25252e%25252e", // Triple URL encoded
		"..%2f",          // Mixed encoding
		"..%5c",          // Windows backslash encoded
		"..%c0%af",       // UTF-8 overlong encoding
		"..%c1%9c",       // UTF-8 overlong encoding variant
		".%2e",           // Partial encoding
		"%2e.",           // Partial encoding variant
		"%2e%2e%2f",      // Full URL encoded traversal
		"%2e%2e%5c",      // Windows variant
		"..\\",           // Windows backslash
		"..\\/",          // Mixed separators
		"..%00",          // Null byte injection
		"..%0a",          // Newline injection
		"..%0d",          // Carriage return injection
		"..%09",          // Tab injection
		"..%20",          // Space injection
		"....//",         // Double dot with double slash
		"....\\\\",       // Double dot with double backslash
		".....",          // Five consecutive dots
		"......",         // Six consecutive dots (defense in depth)
		// Additional patterns for comprehensive security
		"%2E%2E",       // Mixed case URL encoded
		"%2E%2e",       // Mixed case variant
		"%2e%2E",       // Mixed case variant
		"..%2F",        // Mixed case encoding
		"..%5C",        // Mixed case Windows variant
		"%c0%ae",       // UTF-8 overlong encoding for dot
		"%c1%1c",       // UTF-8 overlong encoding variant
		"%c1%9c",       // UTF-8 overlong encoding variant
		"..%255c",      // Double encoded backslash
		"..%c0%af",     // UTF-8 overlong encoded slash
		"..%c1%9c",     // UTF-8 overlong encoded backslash
		"%uff0e%uff0e", // Fullwidth dot encoding
		"..%ef%bc%8f",  // Fullwidth slash encoding
	}

	// Check both original and decoded paths
	for _, pattern := range patterns {
		if indexIgnoreCaseFile(decoded, pattern) != -1 ||
			indexIgnoreCaseFile(path, pattern) != -1 {
			return true
		}
	}

	// Additional check: consecutive dots in any form (3 or more)
	if containsConsecutiveDots(decoded, 3) || containsConsecutiveDots(path, 3) {
		return true
	}

	// Check for encoded null bytes and control characters (case-insensitive)
	encodedNulls := []string{"%00", "%0a", "%0d", "%09", "%20"}
	for _, encoded := range encodedNulls {
		if indexIgnoreCaseFile(decoded, encoded) != -1 ||
			indexIgnoreCaseFile(path, encoded) != -1 {
			return true
		}
	}

	// Check for partial double encoding bypass attempts
	if containsPartialDoubleEncodingIgnoreCase(decoded) ||
		containsPartialDoubleEncodingIgnoreCase(path) {
		return true
	}

	// SECURITY: Check for Unicode lookalike characters that could be used for bypass
	if containsUnicodeLookalikes(decoded) {
		return true
	}

	return false
}

// recursiveURLDecode recursively decodes URL-encoded strings to catch multi-layered obfuscation
func recursiveURLDecode(s string) string {
	decoded := s
	for i := 0; i < 3; i++ { // Maximum 3 levels of decoding
		newDecoded, err := url.PathUnescape(decoded)
		if err != nil || newDecoded == decoded {
			break
		}
		decoded = newDecoded
	}
	return decoded
}

// containsUnicodeLookalikes checks for Unicode characters that look like path traversal characters
// SECURITY: Comprehensive detection of Unicode lookalike variants for defense in depth
func containsUnicodeLookalikes(s string) bool {
	for _, r := range s {
		// Check for fullwidth dots and slashes (common bypass technique)
		switch r {
		// Fullwidth dot variants
		case '\uFF0E', // Fullwidth full stop (looks like .)
			'\u2024', // One dot leader (looks like .)
			'\u2025', // Two dot leader (looks like ..)
			'\u2026': // Horizontal ellipsis (looks like ...)
			return true

		// Fullwidth slash variants
		case '\uFF0F', // Fullwidth solidus (looks like /)
			'\uFF3C', // Fullwidth reverse solidus (looks like \)
			'\u2044', // Fraction slash
			'\u2215', // Division slash
			'\u29F8', // Big solidus
			'\uFE68': // Small reverse solidus
			return true

		// Additional potentially dangerous Unicode characters
		case '\uFF04', // Fullwidth dollar sign (template injection)
			'\uFEFF', // Byte order mark
			'\u2060', // Word joiner
			'\u200B', // Zero-width space
			'\u200C', // Zero-width non-joiner
			'\u200D', // Zero-width joiner
			'\u3000', // Ideographic space
			'\u00AD', // Soft hyphen
			'\u034F', // Combining grapheme joiner
			'\u061C', // Arabic letter mark
			'\u115F', // Korean jamo filler (choseong)
			'\u1160', // Korean jamo filler (jungseong)
			'\u180E': // Mongolian vowel separator
			return true
		}
	}
	return false
}

// indexIgnoreCaseFile finds pattern case-insensitively without allocation
// Delegates to shared implementation in internal package
func indexIgnoreCaseFile(s, pattern string) int {
	return internal.IndexIgnoreCase(s, pattern)
}

// containsPartialDoubleEncodingIgnoreCase checks for partial double encoding bypass attempts (case-insensitive)
func containsPartialDoubleEncodingIgnoreCase(path string) bool {
	patterns := []string{"%2e%2", "%25%2e", "%2f%2", "%5c%2"}
	for _, pattern := range patterns {
		if indexIgnoreCaseFile(path, pattern) != -1 {
			return true
		}
	}
	return false
}

// hasPrefixIgnoreCase checks if s starts with prefix case-insensitively
func hasPrefixIgnoreCase(s, prefix string) bool {
	if len(prefix) > len(s) {
		return false
	}
	for i := 0; i < len(prefix); i++ {
		c1 := s[i]
		c2 := prefix[i]
		if c1 >= 'A' && c1 <= 'Z' {
			c1 += 32
		}
		if c1 != c2 {
			return false
		}
	}
	return true
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

// validateUnixPath validates Unix-specific path security
func validateUnixPath(absPath string) error {
	// Block access to critical system directories using case-insensitive matching
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
		if hasPrefixIgnoreCase(absPath, dir) {
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

	// Additional check for alternate data streams (ADS)
	if strings.Contains(absPath, ":") {
		parts := strings.SplitN(absPath, ":", 2)
		if len(parts) == 2 {
			// Check if it looks like a drive letter pattern
			if len(parts[0]) == 1 && parts[0][0] >= 'A' && parts[0][0] <= 'Z' {
				// This is a drive letter path, not ADS
			} else {
				return newSecurityError("validate_windows_path", "alternate data streams not allowed")
			}
		}
	}

	// Check COM1-9 and LPT1-9
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

	// Check for invalid characters in Windows paths
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
