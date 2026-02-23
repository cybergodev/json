package json

import (
	"fmt"
	"io"
	"os"
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
