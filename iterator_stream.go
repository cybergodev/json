package json

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
)

// sizeLimitedReader caps the total bytes read from an underlying reader,
// returning a clear error when MaxJSONSize is exceeded. Streaming iterators
// wrap their source with this so cfg.MaxJSONSize is honored (consistent with
// LoadFromReader), closing the gap where a stream over a huge array bypassed
// the library's size limit entirely.
type sizeLimitedReader struct {
	r       io.Reader
	remain  int64
	maxSize int64
}

func (l *sizeLimitedReader) Read(p []byte) (int, error) {
	if l.remain <= 0 {
		// Exactly-at-limit input is legal: probe the underlying reader to
		// distinguish a genuinely exhausted source (clean io.EOF) from an
		// oversized one. Previously a stream of exactly MaxJSONSize bytes
		// failed on the decoder's trailing read, and the error was a bare
		// fmt.Errorf instead of the ErrSizeLimit sentinel used by the
		// non-streaming paths (D-002).
		var probe [1]byte
		for {
			n, err := l.r.Read(probe[:])
			if n > 0 {
				return 0, &JsonsError{
					Op:      "stream_read",
					Message: fmt.Sprintf("json: input size exceeds maximum %d bytes", l.maxSize),
					Err:     ErrSizeLimit,
				}
			}
			if err != nil {
				if err == io.EOF {
					return 0, io.EOF
				}
				return 0, err
			}
			// (0, nil) is legal per io.Reader contract; retry the probe.
		}
	}
	if int64(len(p)) > l.remain {
		p = p[:l.remain]
	}
	n, err := l.r.Read(p)
	l.remain -= int64(n)
	return n, err
}

// streamMaxSize resolves the byte cap for a streaming iterator from an optional
// per-call Config, falling back to the default when unset/non-positive.
func streamMaxSize(cfg ...Config) int64 {
	maxSize := int64(DefaultMaxJSONSize)
	if len(cfg) > 0 && cfg[0].MaxJSONSize > 0 {
		maxSize = cfg[0].MaxJSONSize
	}
	return maxSize
}

// ============================================================================
// STREAM ITERATOR - Memory-efficient iteration over large JSON data
// ============================================================================

// StreamIterator provides memory-efficient iteration over large JSON arrays.
// It processes elements one at a time without loading the entire array into memory.
type StreamIterator struct {
	decoder *json.Decoder
	index   int
	err     error
	done    bool
	current any
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

	// Create buffered reader for improved I/O performance. Wrap the source with
	// a size-limited reader so cfg.MaxJSONSize is enforced on the total stream
	// (otherwise an oversized array is read without bound).
	maxSize := streamMaxSize(cfg...)
	limited := &sizeLimitedReader{r: reader, remain: maxSize, maxSize: maxSize}
	buffered := bufio.NewReaderSize(limited, bufSize)
	decoder := json.NewDecoder(buffered)

	return &StreamIterator{
		decoder: decoder,
		index:   -1,
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
			if err == io.EOF {
				// Clean end before any value: an empty stream is NOT an error
				// (D-002) — a bare io.EOF in Err() made callers treat
				// emptiness as failure. Truncation WITHIN the array still
				// surfaces via the element-decode paths below.
				err = nil
			}
			si.err = err
			si.done = true
			return false
		}

		// Handle single value (not an array).
		if token != json.Delim('[') {
			// A non-'[' delimiter here means a top-level object (or stray
			// close) — StreamIterator iterates array elements. The previous
			// code unconditionally followed Token() with Decode(&rest), which
			// for a top-level object read from *inside* it and returned the
			// first key as the value (silently wrong). Reject delimiters; only
			// a scalar top-level value is yielded once as a single element.
			if _, isDelim := token.(json.Delim); isDelim {
				si.err = fmt.Errorf("StreamIterator expects a JSON array, got delimiter %v", token)
				si.done = true
				return false
			}
			si.current = token
			si.index = 0
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

	// Create buffered reader for improved I/O performance. Wrap the source with
	// a size-limited reader so cfg.MaxJSONSize is enforced on the total stream.
	maxSize := streamMaxSize(cfg...)
	limited := &sizeLimitedReader{r: reader, remain: maxSize, maxSize: maxSize}
	buffered := bufio.NewReaderSize(limited, bufSize)
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
			// D-002: a non-object document previously ended iteration with
			// Err() == nil, making corrupted input indistinguishable from a
			// clean end. Surface it as an error.
			soi.err = fmt.Errorf("StreamObjectIterator expects a JSON object, got %v", token)
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
		// D-002: a non-string key is malformed JSON-object input; previously
		// it ended iteration silently with Err() == nil.
		soi.err = fmt.Errorf("StreamObjectIterator: non-string object key %v", key)
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
