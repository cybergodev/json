package json

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// preprocessDataForEncoding normalizes string/[]byte inputs to parsed data
// to prevent double-encoding issues when saving JSON files.
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
