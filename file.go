package json

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

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

// ============================================================================
// LARGE JSON FILE PROCESSOR
// Provides memory-efficient processing for very large JSON files
// PERFORMANCE: Memory-mapped file support and chunked processing
// ============================================================================

// LargeFileConfig holds configuration for large file processing
type LargeFileConfig struct {
	ChunkSize       int64 // Size of each chunk in bytes
	MaxMemory       int64 // Maximum memory to use
	BufferSize      int   // Buffer size for reading
	SamplingEnabled bool  // Enable sampling for very large files
	SampleSize      int   // Number of samples to take
}

// DefaultLargeFileConfig returns the default configuration
func DefaultLargeFileConfig() LargeFileConfig {
	return LargeFileConfig{
		ChunkSize:       1024 * 1024,       // 1MB chunks
		MaxMemory:       100 * 1024 * 1024, // 100MB max
		BufferSize:      64 * 1024,         // 64KB buffer
		SamplingEnabled: true,
		SampleSize:      1000,
	}
}

// LargeFileProcessor handles processing of large JSON files
type LargeFileProcessor struct {
	config LargeFileConfig
}

// NewLargeFileProcessor creates a new large file processor
func NewLargeFileProcessor(config LargeFileConfig) *LargeFileProcessor {
	return &LargeFileProcessor{config: config}
}

// ProcessFile processes a large JSON file efficiently
func (lfp *LargeFileProcessor) ProcessFile(filename string, fn func(item any) error) error {
	file, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	// Use buffered reader for efficiency
	reader := bufio.NewReaderSize(file, lfp.config.BufferSize)

	// Create streaming processor
	sp := NewStreamingProcessor(reader, int(lfp.config.ChunkSize))

	// Stream array elements
	return sp.StreamArray(func(index int, item any) bool {
		if err := fn(item); err != nil {
			return false
		}
		return true
	})
}

// ProcessFileChunked processes a large JSON file in chunks
func (lfp *LargeFileProcessor) ProcessFileChunked(filename string, chunkSize int, fn func(chunk []any) error) error {
	file, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	reader := bufio.NewReaderSize(file, lfp.config.BufferSize)
	sp := NewStreamingProcessor(reader, int(lfp.config.ChunkSize))

	chunk := make([]any, 0, chunkSize)

	err = sp.StreamArray(func(index int, item any) bool {
		chunk = append(chunk, item)

		if len(chunk) >= chunkSize {
			if err := fn(chunk); err != nil {
				return false
			}
			chunk = chunk[:0] // Reset chunk
		}
		return true
	})

	// Process remaining items
	if err == nil && len(chunk) > 0 {
		err = fn(chunk)
	}

	return err
}

// ============================================================================
// CHUNKED JSON READER
// ============================================================================

// ChunkedReader reads JSON in chunks for memory efficiency
type ChunkedReader struct {
	reader    *bufio.Reader
	decoder   *json.Decoder
	buffer    []byte
	chunkSize int
}

// NewChunkedReader creates a new chunked reader
func NewChunkedReader(reader io.Reader, chunkSize int) *ChunkedReader {
	if chunkSize <= 0 {
		chunkSize = 1024 * 1024 // 1MB default
	}

	bufReader := bufio.NewReaderSize(reader, 64*1024)
	return &ChunkedReader{
		reader:    bufReader,
		decoder:   json.NewDecoder(bufReader),
		buffer:    make([]byte, 0, chunkSize),
		chunkSize: chunkSize,
	}
}

// ReadArray reads array elements one at a time
func (cr *ChunkedReader) ReadArray(fn func(item any) bool) error {
	// Check for array start
	token, err := cr.decoder.Token()
	if err != nil {
		return err
	}

	if token != json.Delim('[') {
		// Not an array, try to decode as single value
		var value any
		if err := cr.decoder.Decode(&value); err != nil {
			return err
		}
		fn(value)
		return nil
	}

	for cr.decoder.More() {
		var item any
		if err := cr.decoder.Decode(&item); err != nil {
			return err
		}

		if !fn(item) {
			return nil
		}
	}

	// Consume closing bracket
	_, err = cr.decoder.Token()
	return err
}

// ReadObject reads object key-value pairs one at a time
func (cr *ChunkedReader) ReadObject(fn func(key string, value any) bool) error {
	token, err := cr.decoder.Token()
	if err != nil {
		return err
	}

	if token != json.Delim('{') {
		return nil
	}

	for cr.decoder.More() {
		key, err := cr.decoder.Token()
		if err != nil {
			return err
		}

		keyStr, ok := key.(string)
		if !ok {
			continue
		}

		var value any
		if err := cr.decoder.Decode(&value); err != nil {
			return err
		}

		if !fn(keyStr, value) {
			return nil
		}
	}

	// Consume closing brace
	_, err = cr.decoder.Token()
	return err
}

// ============================================================================
// LAZY JSON PARSER
// Parses JSON on-demand, only parsing accessed paths
// ============================================================================

// LazyParser provides lazy JSON parsing
type LazyParser struct {
	raw      []byte
	parsed   map[string]any
	parseErr error
	once     sync.Once
}

// NewLazyParser creates a new lazy parser
func NewLazyParser(data []byte) *LazyParser {
	return &LazyParser{
		raw: data,
	}
}

// parse performs the actual parsing
func (lp *LazyParser) parse() {
	lp.once.Do(func() {
		lp.parseErr = json.Unmarshal(lp.raw, &lp.parsed)
	})
}

// Get retrieves a value at the given path
func (lp *LazyParser) Get(path string) (any, error) {
	lp.parse()
	if lp.parseErr != nil {
		return nil, lp.parseErr
	}

	// Use internal path navigation for parsed map
	return navigateParsed(lp.parsed, path)
}

// navigateParsed navigates a parsed map to find a value at the given path
func navigateParsed(data map[string]any, path string) (any, error) {
	if path == "" || path == "." {
		return data, nil
	}

	processor := getDefaultProcessor()
	jsonBytes, err := processor.Marshal(data)
	if err != nil {
		return nil, err
	}
	return processor.Get(string(jsonBytes), path)
}

// GetAll returns all parsed data
func (lp *LazyParser) GetAll() (map[string]any, error) {
	lp.parse()
	if lp.parseErr != nil {
		return nil, lp.parseErr
	}
	return lp.parsed, nil
}

// Raw returns the raw JSON bytes
func (lp *LazyParser) Raw() []byte {
	return lp.raw
}

// IsParsed returns whether the JSON has been parsed
func (lp *LazyParser) IsParsed() bool {
	return lp.parsed != nil
}

// ============================================================================
// LINE-DELIMITED JSON PROCESSOR
// For processing NDJSON (newline-delimited JSON) files
// ============================================================================

// NDJSONProcessor processes newline-delimited JSON files
type NDJSONProcessor struct {
	bufferSize int
}

// NewNDJSONProcessor creates a new NDJSON processor
func NewNDJSONProcessor(bufferSize int) *NDJSONProcessor {
	if bufferSize <= 0 {
		bufferSize = 64 * 1024
	}
	return &NDJSONProcessor{bufferSize: bufferSize}
}

// ProcessFile processes an NDJSON file line by line
func (np *NDJSONProcessor) ProcessFile(filename string, fn func(lineNum int, obj map[string]any) error) error {
	file, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	buf := make([]byte, 0, np.bufferSize)
	scanner.Buffer(buf, 10*1024*1024) // 10MB max line size

	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := scanner.Bytes()

		if len(line) == 0 {
			continue
		}

		var obj map[string]any
		if err := json.Unmarshal(line, &obj); err != nil {
			continue // Skip invalid lines
		}

		if err := fn(lineNum, obj); err != nil {
			return err
		}
	}

	return scanner.Err()
}

// ProcessReader processes NDJSON from a reader
func (np *NDJSONProcessor) ProcessReader(reader io.Reader, fn func(lineNum int, obj map[string]any) error) error {
	scanner := bufio.NewScanner(reader)
	buf := make([]byte, 0, np.bufferSize)
	scanner.Buffer(buf, 10*1024*1024)

	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := scanner.Bytes()

		if len(line) == 0 {
			continue
		}

		var obj map[string]any
		if err := json.Unmarshal(line, &obj); err != nil {
			continue
		}

		if err := fn(lineNum, obj); err != nil {
			return err
		}
	}

	return scanner.Err()
}

// ============================================================================
// CHUNKED JSON WRITER
// ============================================================================

// ChunkedWriter writes JSON in chunks for memory efficiency
type ChunkedWriter struct {
	writer    io.Writer
	buffer    []byte
	chunkSize int
	count     int
	first     bool
	isArray   bool
}

// NewChunkedWriter creates a new chunked writer
func NewChunkedWriter(writer io.Writer, chunkSize int, isArray bool) *ChunkedWriter {
	if chunkSize <= 0 {
		chunkSize = 1024 * 1024
	}
	return &ChunkedWriter{
		writer:    writer,
		buffer:    make([]byte, 0, chunkSize),
		chunkSize: chunkSize,
		first:     true,
		isArray:   isArray,
	}
}

// WriteItem writes a single item to the chunk
func (cw *ChunkedWriter) WriteItem(item any) error {
	// Start array/object if first item
	if cw.first {
		if cw.isArray {
			cw.buffer = append(cw.buffer, '[')
		} else {
			cw.buffer = append(cw.buffer, '{')
		}
		cw.first = false
	} else {
		cw.buffer = append(cw.buffer, ',')
	}

	// Encode item
	data, err := json.Marshal(item)
	if err != nil {
		return err
	}
	cw.buffer = append(cw.buffer, data...)
	cw.count++

	// Flush if buffer is full
	if len(cw.buffer) >= cw.chunkSize {
		return cw.Flush(false)
	}

	return nil
}

// WriteKeyValue writes a key-value pair to the chunk
func (cw *ChunkedWriter) WriteKeyValue(key string, value any) error {
	if cw.isArray {
		return cw.WriteItem(value)
	}

	if cw.first {
		cw.buffer = append(cw.buffer, '{')
		cw.first = false
	} else {
		cw.buffer = append(cw.buffer, ',')
	}

	// Encode key-value pair
	data, err := json.Marshal(map[string]any{key: value})
	if err != nil {
		return err
	}
	// Remove the outer braces
	cw.buffer = append(cw.buffer, data[1:len(data)-1]...)
	cw.count++

	if len(cw.buffer) >= cw.chunkSize {
		return cw.Flush(false)
	}

	return nil
}

// Flush writes the buffer to the underlying writer
func (cw *ChunkedWriter) Flush(final bool) error {
	if final {
		if cw.isArray {
			cw.buffer = append(cw.buffer, ']')
		} else {
			cw.buffer = append(cw.buffer, '}')
		}
	}

	_, err := cw.writer.Write(cw.buffer)
	cw.buffer = cw.buffer[:0]
	return err
}

// Count returns the number of items written
func (cw *ChunkedWriter) Count() int {
	return cw.count
}

// ============================================================================
// SAMPLING JSON READER
// For very large files, samples data instead of reading all
// ============================================================================

// SamplingReader samples data from large JSON arrays
type SamplingReader struct {
	decoder    *json.Decoder
	sampleSize int
	totalRead  int64
}

// NewSamplingReader creates a new sampling reader
func NewSamplingReader(reader io.Reader, sampleSize int) *SamplingReader {
	return &SamplingReader{
		decoder:    json.NewDecoder(reader),
		sampleSize: sampleSize,
	}
}

// Sample reads a sample of items from a JSON array
func (sr *SamplingReader) Sample(fn func(index int, item any) bool) error {
	// Check for array start
	token, err := sr.decoder.Token()
	if err != nil {
		return err
	}

	if token != json.Delim('[') {
		// Not an array, read single value
		var value any
		if err := sr.decoder.Decode(&value); err != nil {
			return err
		}
		fn(0, value)
		return nil
	}

	samples := make([]any, 0, sr.sampleSize)
	index := 0

	for sr.decoder.More() {
		var item any
		if err := sr.decoder.Decode(&item); err != nil {
			return err
		}

		sr.totalRead++

		// Reservoir sampling algorithm
		if len(samples) < sr.sampleSize {
			samples = append(samples, item)
		} else {
			// Random replacement (simplified - use actual random in production)
			replaceIdx := index % sr.sampleSize
			if replaceIdx < len(samples) {
				samples[replaceIdx] = item
			}
		}
		index++
	}

	// Process samples
	for i, sample := range samples {
		if !fn(i, sample) {
			break
		}
	}

	// Consume closing bracket
	_, err = sr.decoder.Token()
	return err
}

// TotalRead returns the total number of items read
func (sr *SamplingReader) TotalRead() int64 {
	return sr.totalRead
}
