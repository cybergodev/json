package json

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// LoadFromFile loads JSON data from a file
func (p *Processor) LoadFromFile(filePath string, opts ...*ProcessorOptions) (any, error) {
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
			Op:      "load_from_file",
			Message: fmt.Sprintf("failed to read file %s", filePath),
			Err:     fmt.Errorf("read file error: %w", err), // Modern error wrapping
		}
	}

	// Parse JSON
	var jsonData any
	err = p.Parse(string(data), &jsonData, opts...)
	return jsonData, err
}

// SaveToFile saves data to a JSON file with automatic directory creation
// Parameters:
//   - filePath: file path and name, creates directories if they don't exist
//   - data: JSON data to save
//   - pretty: optional parameter - true for formatted JSON, false for compact JSON (default: false)
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

	// Determine formatting preference
	shouldFormat := false
	if len(pretty) > 0 {
		shouldFormat = pretty[0]
	}

	// Encode data to JSON
	config := DefaultEncodeConfig()
	config.Pretty = shouldFormat
	jsonStr, err := p.EncodeWithConfig(data, config)
	if err != nil {
		return err
	}

	// Write to file
	err = os.WriteFile(filePath, []byte(jsonStr), 0644)
	if err != nil {
		return &JsonsError{
			Op:      "save_to_file",
			Message: fmt.Sprintf("failed to write file %s", filePath),
			Err:     fmt.Errorf("write file error: %w", err), // Modern error wrapping
		}
	}

	return nil
}

// LoadFromReader loads JSON data from an io.Reader with size limits
func (p *Processor) LoadFromReader(reader io.Reader, opts ...*ProcessorOptions) (any, error) {
	if err := p.checkClosed(); err != nil {
		return nil, err
	}

	// Use LimitReader to prevent excessive memory usage
	limitedReader := io.LimitReader(reader, p.config.MaxJSONSize)

	// Read all data
	data, err := io.ReadAll(limitedReader)
	if err != nil {
		return nil, &JsonsError{
			Op:      "load_from_reader",
			Message: "failed to read from reader",
			Err:     fmt.Errorf("reader error: %w", err), // Modern error wrapping
		}
	}

	// Check if we hit the size limit
	if int64(len(data)) >= p.config.MaxJSONSize {
		return nil, &JsonsError{
			Op:      "load_from_reader",
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
func (p *Processor) SaveToWriter(writer io.Writer, data any, pretty bool, opts ...*ProcessorOptions) error {
	if err := p.checkClosed(); err != nil {
		return err
	}

	// Encode data to JSON
	config := DefaultEncodeConfig()
	config.Pretty = pretty
	jsonStr, err := p.EncodeWithConfig(data, config, opts...)
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

// validateFilePath validates file paths for security
func (p *Processor) validateFilePath(filePath string) error {
	if filePath == "" {
		return &JsonsError{
			Op:      "validate_file_path",
			Message: "file path cannot be empty",
			Err:     ErrOperationFailed,
		}
	}

	// Check for path traversal attempts
	if strings.Contains(filePath, "..") {
		return &JsonsError{
			Op:      "validate_file_path",
			Message: "path traversal detected in file path",
			Err:     ErrOperationFailed,
		}
	}

	// Check for null bytes
	if strings.Contains(filePath, "\x00") {
		return &JsonsError{
			Op:      "validate_file_path",
			Message: "null byte detected in file path",
			Err:     ErrOperationFailed,
		}
	}

	// Check for excessively long paths
	if len(filePath) > 4096 {
		return &JsonsError{
			Op:      "validate_file_path",
			Message: "file path too long",
			Err:     ErrOperationFailed,
		}
	}

	// Check for suspicious patterns
	suspiciousPatterns := []string{
		"/dev/", "/proc/", "/sys/", "/etc/passwd", "/etc/shadow",
		"\\\\",
	}

	// Windows reserved device names - check as complete filename components only
	windowsReservedNames := []string{
		"CON", "PRN", "AUX", "NUL",
		"COM1", "COM2", "COM3", "COM4", "COM5", "COM6", "COM7", "COM8", "COM9",
		"LPT1", "LPT2", "LPT3", "LPT4", "LPT5", "LPT6", "LPT7", "LPT8", "LPT9",
	}

	lowerPath := strings.ToLower(filePath)

	// Check general suspicious patterns
	for _, pattern := range suspiciousPatterns {
		if strings.Contains(lowerPath, strings.ToLower(pattern)) {
			return &JsonsError{
				Op:      "validate_file_path",
				Message: fmt.Sprintf("suspicious pattern detected in file path: %s", pattern),
				Err:     ErrOperationFailed,
			}
		}
	}

	// Check Windows reserved names more carefully - only as complete filename components
	// Extract filename from path
	filename := filePath
	if lastSlash := strings.LastIndex(filePath, "/"); lastSlash != -1 {
		filename = filePath[lastSlash+1:]
	}
	if lastBackslash := strings.LastIndex(filename, "\\"); lastBackslash != -1 {
		filename = filename[lastBackslash+1:]
	}

	// Remove extension for reserved name check
	filenameWithoutExt := filename
	if dotIndex := strings.LastIndex(filename, "."); dotIndex != -1 {
		filenameWithoutExt = filename[:dotIndex]
	}

	upperFilename := strings.ToUpper(filenameWithoutExt) // Use ToUpper for case-insensitive comparison
	for _, reserved := range windowsReservedNames {
		if upperFilename == reserved {
			return &JsonsError{
				Op:      "validate_file_path",
				Message: fmt.Sprintf("Windows reserved device name detected in filename: %s", reserved),
				Err:     ErrOperationFailed,
			}
		}
	}

	// Additional security checks
	// Check for path traversal more thoroughly
	if strings.Contains(filePath, "../") || strings.Contains(filePath, "..\\") {
		return &JsonsError{
			Op:      "validate_file_path",
			Message: "path traversal detected in file path",
			Err:     ErrOperationFailed,
		}
	}

	// Check for absolute paths to sensitive system directories
	if strings.HasPrefix(lowerPath, "/etc/") || strings.HasPrefix(lowerPath, "/proc/") ||
	   strings.HasPrefix(lowerPath, "/sys/") || strings.HasPrefix(lowerPath, "/dev/") {
		return &JsonsError{
			Op:      "validate_file_path",
			Message: "access to system directories not allowed",
			Err:     ErrOperationFailed,
		}
	}

	return nil
}
