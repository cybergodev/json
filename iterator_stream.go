package json

import (
	"bufio"
	"encoding/json"
	"io"
)

// ============================================================================
// STREAM ITERATOR - Memory-efficient iteration over large JSON data
// ============================================================================

// StreamIterator provides memory-efficient iteration over large JSON arrays.
// It processes elements one at a time without loading the entire array into memory.
type StreamIterator struct {
	decoder    *json.Decoder
	index      int
	err        error
	done       bool
	current    any
	buffer     *bufio.Reader // Buffered reader for improved I/O performance
	bufferSize int           // Configured buffer size
}

// NewStreamIterator creates a stream iterator from a reader with default settings.
// The optional cfg parameter allows customization using the unified Config pattern.
//
// Example:
//
//	// Default settings
//	iter := json.NewStreamIterator(reader)
//
//	// With custom configuration
//	cfg := json.DefaultConfig()
//	cfg.BufferSize = 64 * 1024
//	iter := json.NewStreamIterator(reader, cfg)
func NewStreamIterator(reader io.Reader, cfg ...Config) *StreamIterator {
	var config Config
	if len(cfg) > 0 {
		config = cfg[0]
	} else {
		config = DefaultConfig()
	}

	// Get buffer size from config, default to 32KB
	bufSize := config.BufferSize
	if bufSize <= 0 {
		bufSize = 32 * 1024
	}

	// Create buffered reader for improved I/O performance
	buffered := bufio.NewReaderSize(reader, bufSize)
	decoder := json.NewDecoder(buffered)

	return &StreamIterator{
		decoder:    decoder,
		index:      -1,
		buffer:     buffered,
		bufferSize: bufSize,
	}
}

// Next advances to the next element
// Returns true if there is a next element, false otherwise
func (si *StreamIterator) Next() bool {
	if si.done || si.err != nil {
		return false
	}

	// First call - check for array start
	if si.index < 0 {
		token, err := si.decoder.Token()
		if err != nil {
			si.err = err
			si.done = true
			return false
		}

		// Handle single value (not an array)
		if token != json.Delim('[') {
			si.current = token
			si.index = 0
			// Try to decode the rest if it's a complex value
			var rest any
			if err := si.decoder.Decode(&rest); err == nil {
				// It was a complex object/array
				si.current = rest
			}
			si.done = true
			return true
		}
	}

	// Check if there are more elements
	if !si.decoder.More() {
		// Consume closing bracket, propagate any decode error
		if _, err := si.decoder.Token(); err != nil {
			si.err = err
		}
		si.done = true
		return false
	}

	// Decode next element
	var item any
	if err := si.decoder.Decode(&item); err != nil {
		si.err = err
		si.done = true
		return false
	}

	si.current = item
	si.index++
	return true
}

// Value returns the current element
func (si *StreamIterator) Value() any {
	return si.current
}

// Index returns the current index
func (si *StreamIterator) Index() int {
	return si.index
}

// Err returns any error encountered during iteration
func (si *StreamIterator) Err() error {
	return si.err
}

// ============================================================================
// STREAM OBJECT ITERATOR - For iterating over JSON objects
// ============================================================================

// StreamObjectIterator provides memory-efficient iteration over JSON objects
type StreamObjectIterator struct {
	decoder *json.Decoder
	key     string
	value   any
	err     error
	done    bool
	started bool
}

// NewStreamObjectIterator creates a stream object iterator from a reader.
// The optional cfg parameter allows customization using the unified Config pattern.
// When config is provided, cfg.BufferSize is used for buffered reading.
//
// Example:
//
//	// Default settings
//	iter := json.NewStreamObjectIterator(reader)
//
//	// With custom buffer size
//	cfg := json.DefaultConfig()
//	cfg.BufferSize = 128 * 1024
//	iter := json.NewStreamObjectIterator(reader, cfg)
func NewStreamObjectIterator(reader io.Reader, cfg ...Config) *StreamObjectIterator {
	var config Config
	if len(cfg) > 0 {
		config = cfg[0]
	} else {
		config = DefaultConfig()
	}

	bufSize := config.BufferSize
	if bufSize <= 0 {
		bufSize = 32 * 1024 // Default buffer size
	}

	// Create buffered reader for improved I/O performance
	buffered := bufio.NewReaderSize(reader, bufSize)
	decoder := json.NewDecoder(buffered)

	return &StreamObjectIterator{
		decoder: decoder,
	}
}

// Next advances to the next key-value pair
func (soi *StreamObjectIterator) Next() bool {
	if soi.done || soi.err != nil {
		return false
	}

	// First call - check for object start
	if !soi.started {
		token, err := soi.decoder.Token()
		if err != nil {
			soi.err = err
			soi.done = true
			return false
		}

		if token != json.Delim('{') {
			soi.done = true
			return false
		}
		soi.started = true
	}

	// Check if there are more elements
	if !soi.decoder.More() {
		// Consume closing brace, propagate any decode error
		if _, err := soi.decoder.Token(); err != nil {
			soi.err = err
		}
		soi.done = true
		return false
	}

	// Read key
	key, err := soi.decoder.Token()
	if err != nil {
		soi.err = err
		soi.done = true
		return false
	}

	keyStr, ok := key.(string)
	if !ok {
		soi.done = true
		return false
	}
	soi.key = keyStr

	// Read value
	var value any
	if err := soi.decoder.Decode(&value); err != nil {
		soi.err = err
		soi.done = true
		return false
	}
	soi.value = value

	return true
}

// Key returns the current key
func (soi *StreamObjectIterator) Key() string {
	return soi.key
}

// Value returns the current value
func (soi *StreamObjectIterator) Value() any {
	return soi.value
}

// Err returns any error encountered
func (soi *StreamObjectIterator) Err() error {
	return soi.err
}
