package json

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"golang.org/x/text/unicode/norm"
)

// LoadFromFile loads JSON data from a file and returns the raw JSON string.
// The file path is validated for security (path traversal, symlinks, etc.).
//
// Errors:
//   - ErrProcessorClosed: processor has been closed
//   - ErrSecurityViolation: path contains traversal or unsafe patterns
//   - ErrSizeLimit: file exceeds MaxJSONSize
//   - File system errors (wrapped in JsonsError)
//
// Example:
//
//	jsonStr, err := processor.LoadFromFile("data.json")
//	if err != nil {
//	    // Handle error
//	}
func (p *Processor) LoadFromFile(filePath string, cfg ...Config) (string, error) {
	data, err := p.readValidatedFile(filePath)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// loadFromFileAsData loads JSON data from a file and returns the parsed data structure.
// This is a convenience method that combines LoadFromFile and Parse.
// The file path is validated for security before reading.
func (p *Processor) loadFromFileAsData(filePath string, cfg ...Config) (any, error) {
	data, err := p.readValidatedFile(filePath)
	if err != nil {
		return nil, err
	}
	var jsonData any
	err = p.Parse(string(data), &jsonData, cfg...)
	return jsonData, err
}

// readValidatedFile validates the file path and reads the file content.
// Shared helper to eliminate duplicate validation+reading code.
// Uses io.LimitReader to enforce size limits during read, preventing TOCTOU races.
func (p *Processor) readValidatedFile(filePath string) ([]byte, error) {
	if err := p.checkClosed(); err != nil {
		return nil, err
	}
	if err := p.validateFilePath(filePath); err != nil {
		return nil, err
	}

	maxSize := p.config.MaxJSONSize
	if maxSize <= 0 {
		maxSize = int64(DefaultMaxJSONSize)
	}

	f, err := os.Open(filePath)
	if err != nil {
		return nil, &JsonsError{
			Op:      "load_from_file",
			Message: fmt.Sprintf("failed to open file: %v", err),
			Err:     err,
		}
	}
	defer func() { _ = f.Close() }() // best-effort cleanup

	// Read up to maxSize+1 bytes: if we get more than maxSize, the file is too large
	limitedReader := io.LimitReader(f, maxSize+1)
	data, err := io.ReadAll(limitedReader)
	if err != nil {
		return nil, &JsonsError{
			Op:      "load_from_file",
			Message: fmt.Sprintf("failed to read file: %v", err),
			Err:     err,
		}
	}
	if int64(len(data)) > maxSize {
		return nil, &JsonsError{
			Op:      "load_from_file",
			Message: fmt.Sprintf("file size exceeds maximum allowed size %d bytes", maxSize),
			Err:     ErrSizeLimit,
		}
	}
	return data, nil
}

// LoadFromReader loads JSON data from an io.Reader and returns the raw JSON string.
// The reader is limited to MaxJSONSize to prevent excessive memory usage.
//
// Errors:
//   - ErrProcessorClosed: processor has been closed
//   - ErrSizeLimit: data exceeds MaxJSONSize
//   - Reader errors (wrapped in JsonsError)
//
// Example:
//
//	file, _ := os.Open("data.json")
//	defer file.Close()
//	jsonStr, err := processor.LoadFromReader(file)
func (p *Processor) LoadFromReader(reader io.Reader, cfg ...Config) (string, error) {
	data, err := p.readValidatedReader(reader)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// loadFromReaderAsData loads JSON data from an io.Reader and returns the parsed data structure.
// This is a convenience method that combines LoadFromReader and Parse.
// Example:
//
//	resp, _ := http.Get(url)
//	defer resp.Body.Close()
//	data, err := processor.LoadFromReaderAsData(resp.Body)
func (p *Processor) loadFromReaderAsData(reader io.Reader, cfg ...Config) (any, error) {
	data, err := p.readValidatedReader(reader)
	if err != nil {
		return nil, err
	}
	var jsonData any
	err = p.Parse(string(data), &jsonData, cfg...)
	return jsonData, err
}

// readValidatedReader reads from a reader with size limiting and validation.
// Shared helper to eliminate duplicate reader validation code.
func (p *Processor) readValidatedReader(reader io.Reader) ([]byte, error) {
	if err := p.checkClosed(); err != nil {
		return nil, err
	}
	// Guard against zero-value MaxJSONSize which would limit reads to 1 byte
	maxSize := p.config.MaxJSONSize
	if maxSize <= 0 {
		maxSize = int64(DefaultMaxJSONSize)
	}
	// Read one byte beyond MaxJSONSize to detect truncation
	limitedReader := io.LimitReader(reader, maxSize+1)
	data, err := io.ReadAll(limitedReader)
	if err != nil {
		return nil, &JsonsError{
			Op:      "load_from_reader",
			Message: fmt.Sprintf("failed to read from reader: %v", err),
			Err:     err,
		}
	}
	// If we read beyond maxSize bytes, the input was truncated
	if int64(len(data)) > maxSize {
		return nil, &JsonsError{
			Op:      "load_from_reader",
			Message: fmt.Sprintf("JSON size exceeds maximum %d bytes", maxSize),
			Err:     ErrSizeLimit,
		}
	}
	return data, nil
}

// ============================================================================
// STREAMING PROCESSING METHODS
// Memory-efficient processing for large JSON files
// ============================================================================

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

// SaveToFile saves data to a JSON file using Config.
// This is the unified API that accepts variadic Config.
// Creates parent directories if they don't exist.
//
// Errors:
//   - ErrProcessorClosed: processor has been closed
//   - ErrSecurityViolation: path contains traversal or unsafe patterns
//   - ErrInvalidJSON: data contains invalid JSON string
//   - File system errors (wrapped in JsonsError)
//
// Example:
//
//	// Simple save
//	err := processor.SaveToFile("data.json", data)
//
//	// Pretty-printed save
//	err := processor.SaveToFile("data.json", data, json.PrettyConfig())
func (p *Processor) SaveToFile(filePath string, data any, cfg ...Config) error {
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
			Message: "failed to create directory for output file",
			Err:     fmt.Errorf("directory creation error: %w", err),
		}
	}

	// Preprocess data to prevent double-encoding of string/[]byte inputs
	processedData, err := p.preprocessDataForEncoding(data)
	if err != nil {
		return err
	}

	// Encode data to JSON
	config := getConfigOrDefault(cfg...)
	jsonStr, err := p.EncodeWithConfig(processedData, config)
	if err != nil {
		return err
	}

	// Write to file
	err = os.WriteFile(filePath, []byte(jsonStr), 0644)
	if err != nil {
		return &JsonsError{
			Op:      "save_to_file",
			Message: fmt.Sprintf("failed to write file: %v", err),
			Err:     err,
		}
	}

	return nil
}

// SaveToWriter saves data to an io.Writer using Config.
// This is the unified API that accepts variadic Config.
//
// Errors:
//   - ErrProcessorClosed: processor has been closed
//   - ErrInvalidJSON: data contains invalid JSON string
//   - Writer errors (wrapped in JsonsError)
//
// Example:
//
//	var buf bytes.Buffer
//	err := processor.SaveToWriter(&buf, data, json.PrettyConfig())
func (p *Processor) SaveToWriter(writer io.Writer, data any, cfg ...Config) error {
	if err := p.checkClosed(); err != nil {
		return err
	}

	// Preprocess data to prevent double-encoding of string/[]byte inputs
	processedData, err := p.preprocessDataForEncoding(data)
	if err != nil {
		return err
	}

	// Encode data to JSON
	config := getConfigOrDefault(cfg...)
	jsonStr, err := p.EncodeWithConfig(processedData, config)
	if err != nil {
		return err
	}

	// Write to writer
	_, err = writer.Write([]byte(jsonStr))
	if err != nil {
		return &JsonsError{
			Op:      "save_to_writer",
			Message: "failed to write to writer",
			Err:     err,
		}
	}

	return nil
}

// MarshalToFile converts data to JSON and saves it to the specified file using Config.
// This is the unified API that accepts variadic Config.
// Creates parent directories if they don't exist.
//
// Errors:
//   - ErrProcessorClosed: processor has been closed
//   - ErrSecurityViolation: path contains traversal or unsafe patterns
//   - ErrInvalidJSON: data cannot be marshaled
//   - File system errors (wrapped in JsonsError)
//
// Example:
//
//	// Simple save
//	err := processor.MarshalToFile("data.json", data)
//
//	// Pretty-printed save
//	err := processor.MarshalToFile("data.json", data, json.PrettyConfig())
func (p *Processor) MarshalToFile(path string, data any, cfg ...Config) error {
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
			Message: "failed to create directory for output file",
			Err:     err,
		}
	}

	// Preprocess data to prevent double-encoding of string/[]byte inputs
	processedData, err := p.preprocessDataForEncoding(data)
	if err != nil {
		return err
	}

	// Determine formatting preference
	config := getConfigOrDefault(cfg...)

	// Marshal data to JSON bytes
	var jsonBytes []byte
	if config.Pretty {
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
			Message: fmt.Sprintf("failed to write file: %v", err),
			Err:     err,
		}
	}

	return nil
}

// UnmarshalFromFile reads JSON data from the specified file and unmarshals it into the provided value.
// The file path is validated for security before reading.
//
// Parameters:
//   - path: file path to read JSON from
//   - v: pointer to the target variable where JSON will be unmarshaled
//   - opts: optional Config for security validation and processing
//
// Errors:
//   - ErrProcessorClosed: processor has been closed
//   - ErrInvalidJSON: file content is not valid JSON
//   - ErrSecurityViolation: path contains traversal or unsafe patterns
//   - ErrSizeLimit: file exceeds MaxJSONSize
//   - File system errors (wrapped in JsonsError)
//
// Example:
//
//	var config Config
//	err := processor.UnmarshalFromFile("config.json", &config)
func (p *Processor) UnmarshalFromFile(path string, v any, cfg ...Config) error {
	if err := p.checkClosed(); err != nil {
		return err
	}

	// Validate input parameters
	if v == nil {
		return &JsonsError{
			Op:      "unmarshal_from_file",
			Message: "unmarshal target cannot be nil",
			Err:     errOperationFailed,
		}
	}

	// Validate file path for security
	if err := p.validateFilePath(path); err != nil {
		return err
	}

	maxSize := p.config.MaxJSONSize
	if maxSize <= 0 {
		maxSize = int64(DefaultMaxJSONSize)
	}

	// Read file contents with size limiting during read
	f, err := os.Open(path)
	if err != nil {
		return &JsonsError{
			Op:      "unmarshal_from_file",
			Message: fmt.Sprintf("failed to open file: %v", err),
			Err:     err,
		}
	}
	defer func() { _ = f.Close() }() // best-effort cleanup

	limitedReader := io.LimitReader(f, maxSize+1)
	data, err := io.ReadAll(limitedReader)
	if err != nil {
		return &JsonsError{
			Op:      "unmarshal_from_file",
			Message: fmt.Sprintf("failed to read file: %v", err),
			Err:     err,
		}
	}

	if int64(len(data)) > maxSize {
		return &JsonsError{
			Op:      "unmarshal_from_file",
			Message: fmt.Sprintf("file size exceeds maximum allowed size %d bytes", maxSize),
			Err:     ErrSizeLimit,
		}
	}

	// Unmarshal JSON data using processor's Unmarshal method
	if err := p.Unmarshal(data, v, cfg...); err != nil {
		return &JsonsError{
			Op:      "unmarshal_from_file",
			Message: fmt.Sprintf("failed to unmarshal JSON: %v", err),
			Err:     err,
		}
	}

	return nil
}

// validateFilePath provides enhanced security validation for file paths.
// Uses smaller helper functions for better maintainability and testability.
func (p *Processor) validateFilePath(filePath string) error {
	// Step 1: Basic validation
	if err := validatePathBasic(filePath); err != nil {
		return err
	}

	// Step 2: Security pattern validation
	if err := validatePathSecurity(filePath); err != nil {
		return err
	}

	// Step 3: Normalize and get absolute path
	absPath, err := normalizeAndAbsPath(filePath)
	if err != nil {
		return err
	}

	// Step 4: Platform-specific validation on absolute path
	if err := validatePathPlatform(absPath); err != nil {
		return err
	}

	// Step 5: Symlink validation
	if err := validatePathSymlinks(absPath); err != nil {
		return err
	}

	// Step 6: File size validation
	return p.validatePathFileSize(absPath)
}

// validatePathBasic performs basic path validation
func validatePathBasic(filePath string) error {
	if filePath == "" {
		return newOperationError("validate_file_path", "file path cannot be empty", errOperationFailed)
	}

	// SECURITY: Check for null bytes before any processing
	if strings.Contains(filePath, "\x00") {
		return newSecurityError("validate_file_path", "null byte in path")
	}

	return nil
}

// validatePathSecurity checks for path traversal and platform-specific security issues
func validatePathSecurity(filePath string) error {
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

	return nil
}

// normalizeAndAbsPath normalizes the path and returns its absolute form
func normalizeAndAbsPath(filePath string) (string, error) {
	// Normalize the path after security checks
	cleanPath := filepath.Clean(filePath)

	// Check path length after cleaning
	if len(cleanPath) > maxPathLength {
		return "", newOperationError("validate_file_path",
			fmt.Sprintf("path too long: %d > %d", len(cleanPath), maxPathLength),
			errOperationFailed)
	}

	// Convert to absolute path for further validation
	absPath, err := filepath.Abs(cleanPath)
	if err != nil {
		return "", newOperationError("validate_file_path", "invalid path", err)
	}

	return absPath, nil
}

// validatePathPlatform performs platform-specific security checks on absolute path
func validatePathPlatform(absPath string) error {
	if runtime.GOOS != "windows" {
		if err := validateUnixPath(absPath); err != nil {
			return err
		}
	}
	return nil
}

// validatePathSymlinks checks for symlink security issues
func validatePathSymlinks(absPath string) error {
	info, err := os.Lstat(absPath)
	if err != nil {
		// File doesn't exist yet, no symlink check needed
		return nil
	}

	if info.Mode()&os.ModeSymlink == 0 {
		// Not a symlink, no check needed
		return nil
	}

	realPath, err := filepath.EvalSymlinks(absPath)
	if err != nil {
		return newOperationError("validate_file_path", "cannot resolve symlink", err)
	}

	// Ensure symlink doesn't escape to restricted areas
	if runtime.GOOS != "windows" {
		return validateUnixPath(realPath)
	}
	return validateWindowsPath(realPath)
}

// validateFilePathStandalone performs security validation without Processor dependency.
// This is used by NDJSONProcessor and other standalone types.
func validateFilePathStandalone(filePath string) error {
	// Step 1: Basic validation
	if err := validatePathBasic(filePath); err != nil {
		return err
	}

	// Step 2: Security pattern validation
	if err := validatePathSecurity(filePath); err != nil {
		return err
	}

	// Step 3: Normalize and get absolute path
	absPath, err := normalizeAndAbsPath(filePath)
	if err != nil {
		return err
	}

	// Step 4: Platform-specific validation on absolute path
	if err := validatePathPlatform(absPath); err != nil {
		return err
	}

	// Step 5: Symlink validation
	return validatePathSymlinks(absPath)
}

// validatePathFileSize checks if file size is within limits
func (p *Processor) validatePathFileSize(absPath string) error {
	info, err := os.Stat(absPath)
	if err != nil {
		// File doesn't exist yet, no size check needed
		return nil
	}

	if info.Size() > p.config.MaxJSONSize {
		return newSizeLimitError("validate_file_path", info.Size(), p.config.MaxJSONSize)
	}
	return nil
}

// containsPathTraversal checks for path traversal patterns comprehensively.
// Uses case-insensitive matching with Unicode normalization and recursive URL decoding.
// NOTE: For JSON path validation, see security.go:validatePathSecurity which provides
// JSON-specific security checks. This function is for file system path validation.
func containsPathTraversal(path string) bool {
	// SECURITY: Apply Unicode NFC normalization to detect homograph attacks
	normalized := norm.NFC.String(path)
	// SECURITY: Recursively decode URL encoding to catch multi-layered obfuscation
	decoded := recursiveURLDecode(normalized)

	// Check both decoded and original for all pattern types
	for _, s := range []string{decoded, path} {
		if containsBasicTraversalPattern(s) || containsEncodedPattern(s) || containsUnicodeLookalike(s) {
			return true
		}
	}
	return false
}

// containsBasicTraversalPattern checks for standalone ".." path components.
func containsBasicTraversalPattern(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] == '.' && i+1 < len(s) && s[i+1] == '.' {
			beforeOK := i == 0 || s[i-1] == '/' || s[i-1] == '\\'
			afterOK := i+2 >= len(s) || s[i+2] == '/' || s[i+2] == '\\'
			if beforeOK && afterOK {
				return true
			}
			i++ // Skip past ".." to avoid false matches
		}
	}
	return containsConsecutiveDots(s, 3)
}

// getTraversalPatterns returns the list of known traversal attack patterns.
// Uses sync.OnceValue for lazy initialization to avoid allocating the slice at package init.
var getTraversalPatterns = sync.OnceValue(func() []string {
	return []string{
		// URL encoded patterns
		"%2e%2e", "%252e%252e", "%25252e%25252e",
		// Mixed encoding patterns
		"..%2f", "..%5c", "..%c0%af", "..%c1%9c",
		// Partial encoding patterns
		".%2e", "%2e.", "%2e%2e%2f", "%2e%2e%5c",
		// Windows patterns
		"..\\", "..\\/",
		// Injection patterns (control chars)
		"..%00", "..%0a", "..%0d", "..%09", "..%20",
		"%00", "%0a", "%0d", "%09", "%20",
		// Double patterns
		"....//", "....\\\\", ".....", "......",
		// Mixed case patterns
		"%2E%2E", "%2E%2e", "%2e%2E", "..%2F", "..%5C",
		// UTF-8 overlong encoding
		"%c0%ae", "%c1%1c", "%c1%9c", "..%255c",
		// Fullwidth encoding
		"%uff0e%uff0e", "..%ef%bc%8f",
		// Partial double encoding
		"%2e%2", "%25%2e", "%2f%2", "%5c%2",
	}
})

// containsEncodedPattern checks for encoded path traversal patterns.
func containsEncodedPattern(s string) bool {
	patterns := getTraversalPatterns()
	for _, pattern := range patterns {
		if fastIndexIgnoreCase(s, pattern) != -1 {
			return true
		}
	}
	return false
}

// recursiveURLDecode recursively decodes URL-encoded strings (max 3 levels).
func recursiveURLDecode(s string) string {
	decoded := s
	for i := 0; i < 3; i++ {
		newDecoded, err := url.PathUnescape(decoded)
		if err != nil || newDecoded == decoded {
			break
		}
		decoded = newDecoded
	}
	return decoded
}

// containsUnicodeLookalike checks for Unicode characters that resemble path separators or dots.
func containsUnicodeLookalike(s string) bool {
	for _, r := range s {
		switch r {
		// Dot lookalikes
		case '\uFF0E', '\u2024', '\u2025', '\u2026':
			return true
		// Slash lookalikes
		case '\uFF0F', '\uFF3C', '\u2044', '\u2215', '\u29F8', '\uFE68':
			return true
		// Dangerous invisible/formatting characters
		case '\uFEFF', '\u2060', '\u200B', '\u200C', '\u200D', '\u3000', '\u00AD', '\u034F', '\u061C', '\u115F', '\u1160', '\u180E':
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
		if c2 >= 'A' && c2 <= 'Z' {
			c2 += 32
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

	// Note: Path traversal (..) is already handled by filepath.Clean() in
	// normalizeAndAbsPath(), so no redundant .. check is needed here.

	return nil
}

// validateWindowsPath validates Windows-specific path security
func validateWindowsPath(absPath string) error {
	// Check for UNC paths
	if strings.HasPrefix(absPath, "\\\\") || strings.HasPrefix(absPath, "//") {
		return newSecurityError("validate_windows_path", "UNC paths not allowed")
	}

	// SECURITY FIX: Comprehensive Alternate Data Streams (ADS) detection
	// ADS format examples: file.txt:stream, C:\path\file.txt:stream
	// Valid Windows paths can have at most 1 colon for drive letter
	// Exception: Drive-relative paths like "C:path\file.txt" are valid
	colonCount := strings.Count(absPath, ":")
	if colonCount > 1 {
		return newSecurityError("validate_windows_path", "alternate data streams not allowed")
	}
	if colonCount == 1 {
		// Check if this is a valid drive letter pattern
		colonIdx := strings.Index(absPath, ":")
		// Drive letter must be at position 1
		if colonIdx == 1 && len(absPath) >= 2 {
			driveLetter := absPath[0]
			if (driveLetter >= 'A' && driveLetter <= 'Z') || (driveLetter >= 'a' && driveLetter <= 'z') {
				// Valid drive letter - both "C:\path" and "C:path" (drive-relative) are allowed
				// This is NOT an ADS
			} else {
				return newSecurityError("validate_windows_path", "alternate data streams not allowed")
			}
		} else if colonIdx == 0 {
			// Colon at position 0 is invalid (e.g., ":stream")
			return newSecurityError("validate_windows_path", "alternate data streams not allowed")
		} else if colonIdx > 1 {
			// Colon not at position 1 (e.g., "file.txt:stream") - this is ADS
			return newSecurityError("validate_windows_path", "alternate data streams not allowed")
		}
	}

	// Extract filename for device name checking
	filename := strings.ToUpper(filepath.Base(absPath))
	if idx := strings.LastIndex(filename, "."); idx > 0 {
		filename = filename[:idx]
	}

	// Check reserved device names (complete list including extended)
	reserved := []string{"CON", "PRN", "AUX", "NUL", "CONIN$", "CONOUT$", "CLOCK$"}
	for _, name := range reserved {
		if filename == name {
			return newSecurityError("validate_windows_path", "Windows reserved device name")
		}
	}

	// Check COM0-9 and LPT0-9 (expanded range with proper validation)
	if len(filename) >= 4 && len(filename) <= 5 {
		prefix := filename[:3]
		suffix := filename[3:]
		if prefix == "COM" || prefix == "LPT" {
			// Check if suffix is a valid number (0-9 for single digit, 10-99 for double)
			validDevice := false
			if len(suffix) == 1 && suffix[0] >= '0' && suffix[0] <= '9' {
				validDevice = true
			} else if len(suffix) == 2 {
				// Allow COM10-COM99, LPT10-LPT99
				if (suffix[0] >= '1' && suffix[0] <= '9') && (suffix[1] >= '0' && suffix[1] <= '9') {
					validDevice = true
				}
			}
			if validDevice {
				return newSecurityError("validate_windows_path", "Windows reserved device name")
			}
		}
	}

	// Check for invalid characters in Windows paths (excluding drive letter portion)
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

// ============================================================================
// LINE-DELIMITED JSON PROCESSOR
// For processing NDJSON (newline-delimited JSON) files
// ============================================================================

// NDJSONProcessor processes newline-delimited JSON files
type NDJSONProcessor struct {
	bufferSize int
	config     Config
}

// NewNDJSONProcessor creates a new NDJSON processor.
// The optional cfg parameter allows customization using the unified Config pattern.
// When config is provided, cfg.JSONLBufferSize is used as the buffer size.
//
// Example:
//
//	// Default settings
//	processor := json.NewNDJSONProcessor()
//
//	// With custom buffer size
//	cfg := json.DefaultConfig()
//	cfg.JSONLBufferSize = 128 * 1024
//	processor := json.NewNDJSONProcessor(cfg)
func NewNDJSONProcessor(cfg ...Config) *NDJSONProcessor {
	var config Config
	if len(cfg) > 0 {
		config = cfg[0]
	} else {
		config = DefaultConfig()
	}

	bufferSize := config.JSONLBufferSize
	if bufferSize <= 0 {
		bufferSize = 64 * 1024 // Default buffer size
	}
	return &NDJSONProcessor{bufferSize: bufferSize, config: config}
}

// ProcessFile processes an NDJSON file line by line
//
// Errors:
//   - ErrSecurityViolation: filename is rejected by path-traversal validation
//   - any error from os.Open (e.g. file not found), or any error from ProcessReader
//     (per-line parse/depth errors or fn errors; see ProcessReader)
func (np *NDJSONProcessor) ProcessFile(filename string, fn func(lineNum int, obj map[string]any) error) error {
	if np == nil {
		return &JsonsError{Op: "ndjson_process", Message: "nil NDJSONProcessor"}
	}
	// SECURITY: Validate file path to prevent path traversal attacks
	if err := validateFilePathStandalone(filename); err != nil {
		return err
	}

	file, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }() // best-effort cleanup; error ignored in defer

	return np.ProcessReader(file, fn)
}

// ProcessReader processes NDJSON from a reader.
// Enforces per-line size limits and nesting depth checks to prevent DoS attacks.
//
// Errors:
//   - a per-line error, wrapped with the line number, when a line exceeds the
//     nesting-depth limit or fails to parse (skipped when JSONLContinueOnErr is set)
//   - any error returned by fn, or while reading reader
//     (including bufio.ErrTooLong when a line exceeds MaxJSONSize)
func (np *NDJSONProcessor) ProcessReader(reader io.Reader, fn func(lineNum int, obj map[string]any) error) error {
	maxLineSize := np.config.MaxJSONSize
	if maxLineSize <= 0 {
		maxLineSize = int64(DefaultMaxJSONSize)
	}

	scanner := bufio.NewScanner(reader)
	buf := make([]byte, 0, np.bufferSize)
	scanner.Buffer(buf, int(maxLineSize)+1)

	maxDepth := np.config.MaxNestingDepthSecurity
	if maxDepth <= 0 {
		maxDepth = DefaultMaxNestingDepth
	}

	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := scanner.Bytes()

		if len(line) == 0 {
			continue
		}

		// SECURITY: Check per-line nesting depth before unmarshaling
		if err := checkNestingDepth(line, maxDepth); err != nil {
			if np.config.JSONLContinueOnErr {
				continue
			}
			return fmt.Errorf("line %d: %w", lineNum, err)
		}

		var obj map[string]any
		if err := json.Unmarshal(line, &obj); err != nil {
			if np.config.JSONLContinueOnErr {
				continue
			}
			return fmt.Errorf("line %d: parse error: %w", lineNum, err)
		}

		if err := fn(lineNum, obj); err != nil {
			return err
		}
	}

	return scanner.Err()
}

// checkNestingDepth validates that a JSON line's nesting depth does not exceed maxDepth.
// This prevents stack overflow from deeply nested JSON structures.
func checkNestingDepth(data []byte, maxDepth int) error {
	depth := 0
	inString := false
	escaped := false
	for _, b := range data {
		if escaped {
			escaped = false
			continue
		}
		if inString {
			if b == '\\' {
				escaped = true
			} else if b == '"' {
				inString = false
			}
			continue
		}
		switch b {
		case '"':
			inString = true
		case '{', '[':
			depth++
			if depth > maxDepth {
				return fmt.Errorf("nesting depth %d exceeds maximum allowed depth %d", depth, maxDepth)
			}
		case '}', ']':
			depth--
		}
	}
	return nil
}

// ============================================================================
// FILE-BASED FOREACH METHODS
// Direct file iteration for convenience
// ============================================================================

// ForeachFile iterates over JSON arrays or objects directly from a file.
// The callback returns an error to signal iteration control:
//   - nil: continue iteration
//   - item.Break(): stop iteration without error
//   - other error: stop iteration and return the error
//
// Example:
//
//	err := processor.ForeachFile("data.json", func(key any, item *json.IterableValue) error {
//	    fmt.Println(item.GetString("name"))
//	    return nil // continue
//	})
//
// Errors:
//   - ErrProcessorClosed: processor has been closed
//   - errors from LoadFromFile (ErrSecurityViolation, file read, ErrSizeLimit, ErrInvalidJSON)
//   - any error returned by fn (item.Break() stops iteration without an error)
func (p *Processor) ForeachFile(filePath string, fn func(key any, item *IterableValue) error) error {
	if err := p.checkClosed(); err != nil {
		return err
	}

	jsonStr, err := p.LoadFromFile(filePath)
	if err != nil {
		return err
	}

	return p.ForeachWithError(jsonStr, ".", fn)
}

// ForeachFileWithPath iterates over JSON arrays or objects at a specific path from a file.
//
// Example:
//
//	err := processor.ForeachFileWithPath("data.json", ".users", func(key any, item *json.IterableValue) error {
//	    fmt.Println(item.GetString("name"))
//	    return nil
//	})
//
// Errors:
//   - ErrProcessorClosed: processor has been closed
//   - errors from LoadFromFile (ErrSecurityViolation, file read, ErrSizeLimit, ErrInvalidJSON)
//   - errors from resolving path (ErrPathNotFound, ErrTypeMismatch)
//   - any error returned by fn (item.Break() stops iteration without an error)
func (p *Processor) ForeachFileWithPath(filePath, path string, fn func(key any, item *IterableValue) error) error {
	if err := p.checkClosed(); err != nil {
		return err
	}

	jsonStr, err := p.LoadFromFile(filePath)
	if err != nil {
		return err
	}

	return p.ForeachWithError(jsonStr, path, fn)
}

// ForeachFileChunked iterates over JSON arrays from a file in chunks (batches).
// This is useful for batch processing large datasets.
//
// Example:
//
//	err := processor.ForeachFileChunked("data.json", 100, func(chunk []*json.IterableValue) error {
//	    // Process batch of 100 items
//	    for _, item := range chunk {
//	        fmt.Println(item.GetString("name"))
//	    }
//	    return nil
//	})
//
// Errors:
//   - ErrProcessorClosed: processor has been closed
//   - errors from LoadFromFile (ErrSecurityViolation, file read, ErrSizeLimit, ErrInvalidJSON)
//   - ErrTypeMismatch: the root value is not a JSON array
//   - any error returned by fn (item.Break() stops iteration without an error)
func (p *Processor) ForeachFileChunked(filePath string, chunkSize int, fn func(chunk []*IterableValue) error) error {
	if err := p.checkClosed(); err != nil {
		return err
	}

	if chunkSize <= 0 {
		chunkSize = 100
	}

	jsonStr, err := p.LoadFromFile(filePath)
	if err != nil {
		return err
	}

	data, err := p.Get(jsonStr, ".")
	if err != nil {
		return err
	}

	arr, ok := data.([]any)
	if !ok {
		return &JsonsError{
			Op:      "foreach_file_chunked",
			Message: "expected JSON array at root for chunked iteration",
			Err:     ErrTypeMismatch,
		}
	}

	chunk := make([]*IterableValue, 0, chunkSize)
	for _, item := range arr {
		iv := iterableValuePool.Get().(*IterableValue)
		iv.data = item
		chunk = append(chunk, iv)

		if len(chunk) >= chunkSize {
			if err := fn(chunk); err != nil {
				releaseIterableValues(chunk)
				if errors.Is(err, errBreak) {
					return nil
				}
				return err
			}
			releaseIterableValues(chunk)
			chunk = chunk[:0] // reset slice
		}
	}

	// Process remaining items
	if len(chunk) > 0 {
		if err := fn(chunk); err != nil {
			releaseIterableValues(chunk)
			if errors.Is(err, errBreak) {
				return nil
			}
			return err
		}
		releaseIterableValues(chunk)
	}

	return nil
}

// ForeachFileNested recursively iterates over all nested JSON structures from a file.
//
// Example:
//
//	err := processor.ForeachFileNested("data.json", func(key any, item *json.IterableValue) error {
//	    fmt.Printf("Key: %v, Type: %T\n", key, item.Value)
//	    return nil
//	})
//
// Errors:
//   - ErrProcessorClosed: processor has been closed
//   - errors from LoadFromFile (ErrSecurityViolation, file read, ErrSizeLimit, ErrInvalidJSON)
//   - any error returned by fn (item.Break() stops iteration without an error)
func (p *Processor) ForeachFileNested(filePath string, fn func(key any, item *IterableValue) error) error {
	if err := p.checkClosed(); err != nil {
		return err
	}

	jsonStr, err := p.LoadFromFile(filePath)
	if err != nil {
		return err
	}

	return p.ForeachNestedWithError(jsonStr, fn)
}

// ============================================================================
// Package-level file iteration wrappers (dual-layer design)
// ============================================================================

// ForeachFile iterates over JSON arrays or objects directly from a file.
// The callback returns an error to signal iteration control:
//   - nil: continue iteration
//   - item.Break(): stop iteration without error
//   - other error: stop iteration and return the error
//
// Example:
//
//	err := json.ForeachFile("data.json", func(key any, item *json.IterableValue) error {
//	    fmt.Println(item.GetString("name"))
//	    return nil // continue
//	})
//
// Errors: see Processor.ForeachFile.
func ForeachFile(filePath string, fn func(key any, item *IterableValue) error) error {
	return withProcessorError(func(p *Processor) error {
		return p.ForeachFile(filePath, fn)
	})
}

// ForeachFileWithPath iterates over JSON arrays or objects at a specific path from a file.
//
// Example:
//
//	err := json.ForeachFileWithPath("data.json", ".users", func(key any, item *json.IterableValue) error {
//	    fmt.Println(item.GetString("name"))
//	    return nil
//	})
//
// Errors: see Processor.ForeachFileWithPath.
func ForeachFileWithPath(filePath, path string, fn func(key any, item *IterableValue) error) error {
	return withProcessorError(func(p *Processor) error {
		return p.ForeachFileWithPath(filePath, path, fn)
	})
}

// ForeachFileChunked iterates over JSON arrays from a file in chunks (batches).
// This is useful for batch processing large datasets.
//
// Example:
//
//	err := json.ForeachFileChunked("data.json", 100, func(chunk []*json.IterableValue) error {
//	    // Process batch of 100 items
//	    for _, item := range chunk {
//	        fmt.Println(item.GetString("name"))
//	    }
//	    return nil
//	})
//
// Errors: see Processor.ForeachFileChunked.
func ForeachFileChunked(filePath string, chunkSize int, fn func(chunk []*IterableValue) error) error {
	return withProcessorError(func(p *Processor) error {
		return p.ForeachFileChunked(filePath, chunkSize, fn)
	})
}

// ForeachFileNested recursively iterates over all nested JSON structures from a file.
//
// Example:
//
//	err := json.ForeachFileNested("data.json", func(key any, item *json.IterableValue) error {
//	    fmt.Printf("Key: %v, Type: %T\n", key, item.Value)
//	    return nil
//	})
//
// Errors: see Processor.ForeachFileNested.
func ForeachFileNested(filePath string, fn func(key any, item *IterableValue) error) error {
	return withProcessorError(func(p *Processor) error {
		return p.ForeachFileNested(filePath, fn)
	})
}
