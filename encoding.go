package json

import (
	"bufio"
	"bytes"
	"encoding"
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

// marshalJSON marshals a value to JSON string with optional pretty printing
// This helper function consolidates duplicate marshaling logic
func marshalJSON(value any, pretty bool, prefix, indent string) (string, error) {
	var resultBytes []byte
	var err error

	if pretty {
		resultBytes, err = json.MarshalIndent(value, prefix, indent)
	} else {
		resultBytes, err = json.Marshal(value)
	}

	if err != nil {
		return "", err
	}

	return string(resultBytes), nil
}

// EncodeWithOptions converts any Go value to JSON string with advanced options
func (p *Processor) EncodeWithOptions(value any, encOpts *EncodeConfig, opts ...*ProcessorOptions) (string, error) {
	if err := p.checkClosed(); err != nil {
		return "", err
	}

	if encOpts == nil {
		encOpts = DefaultEncodeConfig()
	}

	// Valid depth
	if encOpts.MaxDepth > 0 {
		if err := p.validateDepth(value, encOpts.MaxDepth, 0); err != nil {
			return "", err
		}
	}

	var result string
	var err error

	// Check if we need to use custom encoding features
	needsCustomEncoding := needsCustomEncodingOpts(encOpts)

	if needsCustomEncoding {
		// Use custom encoder for advanced options
		encoder := NewCustomEncoder(encOpts)
		defer encoder.Close() // Ensure buffers are returned to pool
		result, err = encoder.Encode(value)
	} else {
		// Use standard JSON encoding for basic options
		result, err = marshalJSON(value, encOpts.Pretty, encOpts.Prefix, encOpts.Indent)
	}

	if err != nil {
		return "", &JsonsError{
			Op:      "encode_with_options",
			Message: fmt.Sprintf("failed to encode value: %v", err),
			Err:     ErrOperationFailed,
		}
	}

	// Check size limit
	if int64(len(result)) > p.config.MaxJSONSize {
		return "", &JsonsError{
			Op:      "encode_with_options",
			Message: fmt.Sprintf("encoded JSON size %d exceeds maximum %d", len(result), p.config.MaxJSONSize),
			Err:     ErrSizeLimit,
		}
	}

	return result, nil
}

// validateDepth checks if the data structure exceeds maximum depth
func (p *Processor) validateDepth(value any, maxDepth, currentDepth int) error {
	if currentDepth > maxDepth {
		return &JsonsError{
			Op:      "validate_depth",
			Message: fmt.Sprintf("data structure depth %d exceeds maximum %d", currentDepth, maxDepth),
			Err:     ErrOperationFailed,
		}
	}

	switch v := value.(type) {
	case map[string]any:
		for _, val := range v {
			if err := p.validateDepth(val, maxDepth, currentDepth+1); err != nil {
				return err
			}
		}
	case []any:
		for _, val := range v {
			if err := p.validateDepth(val, maxDepth, currentDepth+1); err != nil {
				return err
			}
		}
	case map[any]any:
		for _, val := range v {
			if err := p.validateDepth(val, maxDepth, currentDepth+1); err != nil {
				return err
			}
		}
	}

	return nil
}

// needsCustomEncodingOpts checks if the encoding options require custom encoding logic
func needsCustomEncodingOpts(cfg *EncodeConfig) bool {
	return cfg.DisableEscaping ||
		cfg.EscapeUnicode ||
		cfg.EscapeSlash ||
		!cfg.EscapeNewlines ||  // When false, need custom encoding to NOT escape
		!cfg.EscapeTabs ||      // When false, need custom encoding to NOT escape
		cfg.CustomEscapes != nil ||
		cfg.SortKeys ||
		cfg.EscapeHTML ||      // When true, need custom encoding to escape HTML (std lib doesn't)
		cfg.FloatPrecision >= 0 ||
		!cfg.IncludeNulls
}

// ToJsonString converts any Go value to JSON string with HTML escaping (safe for web)
func (p *Processor) ToJsonString(value any, opts ...*ProcessorOptions) (string, error) {
	config := DefaultEncodeConfig()
	config.Pretty = false
	config.EscapeHTML = true
	return p.EncodeWithConfig(value, config, opts...)
}

// ToJsonStringPretty converts any Go value to pretty JSON string with HTML escaping
func (p *Processor) ToJsonStringPretty(value any, opts ...*ProcessorOptions) (string, error) {
	config := DefaultEncodeConfig()
	config.Pretty = true
	config.EscapeHTML = true
	return p.EncodeWithConfig(value, config, opts...)
}

// ToJsonStringStandard converts any Go value to compact JSON string without HTML escaping
func (p *Processor) ToJsonStringStandard(value any, opts ...*ProcessorOptions) (string, error) {
	return p.EncodeWithConfig(value, DefaultEncodeConfig(), opts...)
}

// Marshal converts any Go value to JSON bytes (similar to json.Marshal)
func (p *Processor) Marshal(value any, opts ...*ProcessorOptions) ([]byte, error) {
	jsonStr, err := p.ToJsonString(value, opts...)
	if err != nil {
		return nil, err
	}
	return []byte(jsonStr), nil
}

// MarshalIndent converts any Go value to indented JSON bytes (similar to json.MarshalIndent)
func (p *Processor) MarshalIndent(value any, prefix, indent string, opts ...*ProcessorOptions) ([]byte, error) {
	encOpts := DefaultEncodeConfig()
	encOpts.Pretty = true
	encOpts.Prefix = prefix
	encOpts.Indent = indent

	jsonStr, err := p.EncodeWithConfig(value, encOpts, opts...)
	if err != nil {
		return nil, err
	}
	return []byte(jsonStr), nil
}

// Unmarshal parses the JSON-encoded data and stores the result in the value pointed to by v.
// This method is fully compatible with encoding/json.Unmarshal.
func (p *Processor) Unmarshal(data []byte, v any, opts ...*ProcessorOptions) error {
	if err := p.checkClosed(); err != nil {
		return err
	}

	if v == nil {
		return &InvalidUnmarshalError{Type: nil}
	}

	// Convert bytes to string for internal processing
	jsonStr := string(data)

	// Use the existing Parse method which handles all the validation and parsing logic
	return p.Parse(jsonStr, v, opts...)
}

// EncodeStream encodes multiple values as a JSON array stream
func (p *Processor) EncodeStream(values any, pretty bool, opts ...*ProcessorOptions) (string, error) {
	if err := p.checkClosed(); err != nil {
		return "", err
	}

	// Encode as array
	config := DefaultEncodeConfig()
	config.Pretty = pretty
	return p.EncodeWithConfig(values, config, opts...)
}

// EncodeStreamWithOptions encodes multiple values as a JSON array stream with advanced options
func (p *Processor) EncodeStreamWithOptions(values any, encOpts *EncodeConfig, opts ...*ProcessorOptions) (string, error) {
	if err := p.checkClosed(); err != nil {
		return "", err
	}

	// Encode as array with options
	return p.EncodeWithConfig(values, encOpts, opts...)
}

// EncodeBatch encodes multiple key-value pairs as a JSON object
func (p *Processor) EncodeBatch(pairs map[string]any, pretty bool, opts ...*ProcessorOptions) (string, error) {
	config := DefaultEncodeConfig()
	config.Pretty = pretty
	return p.EncodeWithConfig(pairs, config, opts...)
}

// EncodeFields encodes struct fields selectively based on field names
func (p *Processor) EncodeFields(value any, fields []string, pretty bool, opts ...*ProcessorOptions) (string, error) {
	processor := p

	// First convert to JSON and parse back to get map representation
	config := DefaultEncodeConfig()
	config.Pretty = false
	tempJSON, err := processor.EncodeWithConfig(value, config, opts...)
	if err != nil {
		return "", err
	}

	// Parse to any and convert to map
	var anyData any
	err = processor.Parse(tempJSON, &anyData, opts...)
	if err != nil {
		return "", err
	}

	// Check if the result is actually a map
	data, ok := anyData.(map[string]any)
	if !ok {
		return "", &JsonsError{
			Op:      "encode_fields",
			Message: fmt.Sprintf("JSON is not an object, got %T", anyData),
			Err:     ErrTypeMismatch,
		}
	}

	// Filter fields
	filtered := make(map[string]any)
	for _, field := range fields {
		if val, exists := data[field]; exists {
			filtered[field] = val
		}
	}

	finalConfig := DefaultEncodeConfig()
	finalConfig.Pretty = pretty
	return processor.EncodeWithConfig(filtered, finalConfig, opts...)
}

// EncodeWithConfig converts any Go value to JSON string with full configuration control
func (p *Processor) EncodeWithConfig(value any, config *EncodeConfig, opts ...*ProcessorOptions) (string, error) {
	if err := p.checkClosed(); err != nil {
		return "", err
	}

	if config == nil {
		config = DefaultEncodeConfig()
	}

	// Valid depth
	if config.MaxDepth > 0 {
		if err := p.validateDepth(value, config.MaxDepth, 0); err != nil {
			return "", err
		}
	}

	var result string
	var err error

	// Check if we need to use custom encoding features
	needsCustomEncoding := needsCustomEncodingOpts(config)

	if needsCustomEncoding {
		// Use custom encoder for advanced options
		encoder := NewCustomEncoder(config)
		defer encoder.Close() // Ensure buffers are returned to pool
		result, err = encoder.Encode(value)
	} else {
		// Use standard JSON encoding for basic options
		result, err = marshalJSON(value, config.Pretty, config.Prefix, config.Indent)
	}

	if err != nil {
		return "", &JsonsError{
			Op:      "encode_with_config",
			Message: fmt.Sprintf("failed to encode value: %v", err),
			Err:     ErrOperationFailed,
		}
	}

	// Check size limit
	if int64(len(result)) > p.config.MaxJSONSize {
		return "", &JsonsError{
			Op:      "encode_with_config",
			Message: fmt.Sprintf("encoded JSON size %d exceeds maximum %d", len(result), p.config.MaxJSONSize),
			Err:     ErrSizeLimit,
		}
	}

	return result, nil
}

// Encode converts any Go value to JSON string
// This is a convenience method that matches the package-level Encode signature
func (p *Processor) Encode(value any, config ...*EncodeConfig) (string, error) {
	var cfg *EncodeConfig
	if len(config) > 0 {
		cfg = config[0]
	}
	return p.EncodeWithConfig(value, cfg)
}

// EncodePretty converts any Go value to pretty-formatted JSON string
// This is a convenience method that matches the package-level EncodePretty signature
func (p *Processor) EncodePretty(value any, config ...*EncodeConfig) (string, error) {
	var cfg *EncodeConfig
	if len(config) > 0 && config[0] != nil {
		cfg = config[0]
	} else {
		cfg = NewPrettyConfig()
	}
	return p.EncodeWithConfig(value, cfg)
}

// Buffer pools for custom encoder memory optimization
var (
	encoderBufferPool = sync.Pool{
		New: func() any {
			buf := &bytes.Buffer{}
			buf.Grow(2048)
			return buf
		},
	}
)

func getEncoderBuffer() *bytes.Buffer {
	buf := encoderBufferPool.Get().(*bytes.Buffer)
	buf.Reset()
	return buf
}

func putEncoderBuffer(buf *bytes.Buffer) {
	const maxPoolBufferSize = 8 * 1024
	const minPoolBufferSize = 256
	if buf != nil {
		c := buf.Cap()
		if c >= minPoolBufferSize && c <= maxPoolBufferSize {
			buf.Reset()
			encoderBufferPool.Put(buf)
		}
	}
}

// CustomEncoder provides advanced JSON encoding with configurable options
type CustomEncoder struct {
	config      *EncodeConfig
	buffer      *bytes.Buffer
	depth       int
	keyBuffer   *bytes.Buffer
	valueBuffer *bytes.Buffer
}

// NewCustomEncoder creates a new custom encoder with the given configuration
func NewCustomEncoder(config *EncodeConfig) *CustomEncoder {
	if config == nil {
		config = DefaultEncodeConfig()
	}
	return &CustomEncoder{
		config:      config,
		buffer:      getEncoderBuffer(),
		keyBuffer:   getEncoderBuffer(),
		valueBuffer: getEncoderBuffer(),
		depth:       0,
	}
}

// Close releases the encoder's buffers back to the pool
func (e *CustomEncoder) Close() {
	if e.buffer != nil {
		putEncoderBuffer(e.buffer)
		e.buffer = nil
	}
	if e.keyBuffer != nil {
		putEncoderBuffer(e.keyBuffer)
		e.keyBuffer = nil
	}
	if e.valueBuffer != nil {
		putEncoderBuffer(e.valueBuffer)
		e.valueBuffer = nil
	}
}

// Encode encodes the given value to JSON string using custom options
func (e *CustomEncoder) Encode(value any) (string, error) {
	e.buffer.Reset()
	e.keyBuffer.Reset()
	e.valueBuffer.Reset()
	e.depth = 0

	if err := e.encodeValue(value); err != nil {
		return "", err
	}

	return e.buffer.String(), nil
}

// encodeValue encodes any value recursively
func (e *CustomEncoder) encodeValue(value any) error {
	if e.depth > e.config.MaxDepth {
		return &JsonsError{
			Op:      "custom_encode",
			Message: fmt.Sprintf("encoding depth %d exceeds maximum %d", e.depth, e.config.MaxDepth),
			Err:     ErrDepthLimit,
		}
	}

	if value == nil {
		e.buffer.WriteString("null")
		return nil
	}

	v := reflect.ValueOf(value)

	// Handle pointers
	for v.Kind() == reflect.Ptr {
		if v.IsNil() {
			e.buffer.WriteString("null")
			return nil
		}
		v = v.Elem()
	}

	// Check if the value implements json.Marshaler interface first
	if marshaler, ok := value.(Marshaler); ok {
		data, err := marshaler.MarshalJSON()
		if err != nil {
			return &MarshalerError{
				Type:       reflect.TypeOf(value),
				Err:        err,
				sourceFunc: "MarshalJSON",
			}
		}
		e.buffer.Write(data)
		return nil
	}

	// Check if the value implements encoding.TextMarshaler interface
	if textMarshaler, ok := value.(encoding.TextMarshaler); ok {
		text, err := textMarshaler.MarshalText()
		if err != nil {
			return &MarshalerError{
				Type:       reflect.TypeOf(value),
				Err:        err,
				sourceFunc: "MarshalText",
			}
		}
		return e.encodeString(string(text))
	}

	// Handle json.Number type specially to preserve original format
	if jsonNum, ok := value.(json.Number); ok {
		return e.encodeJSONNumber(jsonNum)
	}

	// Handle time.Time type specially to convert to RFC3339 string
	if timeVal, ok := value.(time.Time); ok {
		return e.encodeString(timeVal.Format(time.RFC3339))
	}
	if timeVal, ok := v.Interface().(time.Time); ok {
		return e.encodeString(timeVal.Format(time.RFC3339))
	}

	switch v.Kind() {
	case reflect.Bool:
		if v.Bool() {
			e.buffer.WriteString("true")
		} else {
			e.buffer.WriteString("false")
		}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		e.buffer.WriteString(strconv.FormatInt(v.Int(), 10))
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		e.buffer.WriteString(strconv.FormatUint(v.Uint(), 10))
	case reflect.Float32, reflect.Float64:
		return e.encodeFloat(v.Float(), v.Type().Bits())
	case reflect.String:
		return e.encodeString(v.String())
	case reflect.Array, reflect.Slice:
		return e.encodeArray(v)
	case reflect.Map:
		return e.encodeMap(v)
	case reflect.Struct:
		return e.encodeStruct(v)
	case reflect.Interface:
		return e.encodeValue(v.Interface())
	default:
		// Fallback to standard JSON encoding
		data, err := json.Marshal(value)
		if err != nil {
			return err
		}
		e.buffer.Write(data)
	}

	return nil
}

// encodeJSONNumber encodes json.Number while preserving original format
func (e *CustomEncoder) encodeJSONNumber(num json.Number) error {
	numStr := string(num)

	// If PreserveNumbers is enabled, keep the original string representation
	if e.config.PreserveNumbers {
		e.buffer.WriteString(numStr)
		return nil
	}

	// Otherwise, try to convert to appropriate Go type
	// Check if it's an integer (no decimal point and no scientific notation)
	if !strings.Contains(numStr, ".") && !strings.ContainsAny(numStr, "eE") {
		// Integer format
		if i, err := num.Int64(); err == nil {
			e.buffer.WriteString(strconv.FormatInt(i, 10))
			return nil
		}
	}

	// Float format
	if f, err := num.Float64(); err == nil {
		return e.encodeFloat(f, 64)
	}

	// Fallback: use original string
	e.buffer.WriteString(numStr)
	return nil
}

func (e *CustomEncoder) encodeFloat(f float64, bits int) error {
	if e.config.FloatPrecision >= 0 {
		formatted := strconv.FormatFloat(f, 'f', e.config.FloatPrecision, bits)
		e.buffer.WriteString(formatted)
		return nil
	}

	if f >= -1e15 && f <= 1e15 {
		formatted := strconv.FormatFloat(f, 'f', -1, bits)
		e.buffer.WriteString(formatted)
	} else {
		formatted := strconv.FormatFloat(f, 'g', -1, bits)
		e.buffer.WriteString(formatted)
	}

	return nil
}

func (e *CustomEncoder) encodeString(s string) error {
	e.buffer.WriteByte('"')

	if e.config.DisableEscaping {
		for i := 0; i < len(s); i++ {
			b := s[i]
			switch b {
			case '"':
				e.buffer.WriteString(`\"`)
			case '\\':
				e.buffer.WriteString(`\\`)
			default:
				if b < 0x80 {
					e.buffer.WriteByte(b)
				} else {
					r, size := utf8.DecodeRuneInString(s[i:])
					e.buffer.WriteRune(r)
					i += size - 1
				}
			}
		}
	} else {
		for _, r := range s {
			if err := e.escapeRune(r); err != nil {
				return err
			}
		}
	}

	e.buffer.WriteByte('"')
	return nil
}

func (e *CustomEncoder) escapeRune(r rune) error {
	if e.config.CustomEscapes != nil {
		if escape, exists := e.config.CustomEscapes[r]; exists {
			e.buffer.WriteString(escape)
			return nil
		}
	}

	switch r {
	case '"':
		e.buffer.WriteString(`\"`)
	case '\\':
		e.buffer.WriteString(`\\`)
	case '\b':
		e.buffer.WriteString(`\b`)
	case '\f':
		e.buffer.WriteString(`\f`)
	case '\n':
		if e.config.EscapeNewlines {
			e.buffer.WriteString(`\n`)
		} else {
			e.buffer.WriteRune(r)
		}
	case '\r':
		e.buffer.WriteString(`\r`)
	case '\t':
		if e.config.EscapeTabs {
			e.buffer.WriteString(`\t`)
		} else {
			e.buffer.WriteRune(r)
		}
	case '/':
		if e.config.EscapeSlash {
			e.buffer.WriteString(`\/`)
		} else {
			e.buffer.WriteRune(r)
		}
	default:
		if r < 0x20 {
			e.buffer.WriteString(fmt.Sprintf(`\u%04x`, r))
		} else if e.config.EscapeHTML && (r == '<' || r == '>' || r == '&') {
			e.buffer.WriteString(fmt.Sprintf(`\u%04x`, r))
		} else if e.config.EscapeUnicode && r > 0x7F {
			e.buffer.WriteString(fmt.Sprintf(`\u%04x`, r))
		} else if !utf8.ValidRune(r) && e.config.ValidateUTF8 {
			return &JsonsError{
				Op:      "escape_rune",
				Message: fmt.Sprintf("invalid UTF-8 rune: %U", r),
				Err:     ErrOperationFailed,
			}
		} else {
			e.buffer.WriteRune(r)
		}
	}

	return nil
}

func (e *CustomEncoder) encodeArray(v reflect.Value) error {
	e.buffer.WriteByte('[')
	e.depth++

	length := v.Len()
	for i := 0; i < length; i++ {
		if i > 0 {
			e.buffer.WriteByte(',')
		}

		if e.config.Pretty {
			e.writeIndent()
		}

		if err := e.encodeValue(v.Index(i).Interface()); err != nil {
			return err
		}
	}

	e.depth--
	if e.config.Pretty && length > 0 {
		e.writeIndent()
	}
	e.buffer.WriteByte(']')

	return nil
}

func (e *CustomEncoder) encodeMap(v reflect.Value) error {
	e.buffer.WriteByte('{')
	e.depth++

	keys := v.MapKeys()
	if e.config.SortKeys {
		sort.Slice(keys, func(i, j int) bool {
			return keys[i].String() < keys[j].String()
		})
	}

	first := true
	for _, key := range keys {
		value := v.MapIndex(key)

		if !e.config.IncludeNulls && (value.Interface() == nil || (value.Kind() == reflect.Ptr && value.IsNil())) {
			continue
		}

		if !first {
			e.buffer.WriteByte(',')
		}
		first = false

		if e.config.Pretty {
			e.writeIndent()
		}

		if err := e.encodeString(key.String()); err != nil {
			return err
		}

		e.buffer.WriteByte(':')
		if e.config.Pretty {
			e.buffer.WriteByte(' ')
		}

		if err := e.encodeValue(value.Interface()); err != nil {
			return err
		}
	}

	e.depth--
	if e.config.Pretty && len(keys) > 0 {
		e.writeIndent()
	}
	e.buffer.WriteByte('}')

	return nil
}

func (e *CustomEncoder) encodeStruct(v reflect.Value) error {
	// Use custom encoding when any of these advanced features are enabled
	if !e.config.IncludeNulls || e.config.SortKeys || !e.config.EscapeHTML ||
		e.config.FloatPrecision >= 0 || !e.config.EscapeNewlines || !e.config.EscapeTabs ||
		e.config.EscapeSlash || e.config.EscapeUnicode {
		return e.encodeStructCustom(v)
	}

	if e.config.Pretty {
		data, err := json.MarshalIndent(v.Interface(), e.config.Prefix, e.config.Indent)
		if err != nil {
			return err
		}
		e.buffer.Write(data)
		return nil
	}

	data, err := json.Marshal(v.Interface())
	if err != nil {
		return err
	}
	e.buffer.Write(data)
	return nil
}

func (e *CustomEncoder) encodeStructCustom(v reflect.Value) error {
	e.buffer.WriteByte('{')
	e.depth++

	t := v.Type()
	var fields []reflect.StructField
	var fieldValues []reflect.Value

	for i := 0; i < v.NumField(); i++ {
		field := t.Field(i)
		fieldValue := v.Field(i)

		if !field.IsExported() {
			continue
		}

		jsonTag := field.Tag.Get("json")
		if jsonTag == "-" {
			continue
		}

		tagParts := strings.Split(jsonTag, ",")

		hasOmitEmpty := false
		for _, part := range tagParts[1:] {
			if part == "omitempty" {
				hasOmitEmpty = true
				break
			}
		}

		shouldSkip := false

		// Only respect struct omitempty tags for empty field handling
		if hasOmitEmpty && e.isEmpty(fieldValue) {
			shouldSkip = true
		}

		if !e.config.IncludeNulls {
			if fieldValue.Interface() == nil || (fieldValue.Kind() == reflect.Ptr && fieldValue.IsNil()) {
				shouldSkip = true
			}
		}

		if !shouldSkip {
			fields = append(fields, field)
			fieldValues = append(fieldValues, fieldValue)
		}
	}

	if e.config.SortKeys {
		indices := make([]int, len(fields))
		for i := range indices {
			indices[i] = i
		}

		sort.Slice(indices, func(i, j int) bool {
			nameI := fields[indices[i]].Name
			nameJ := fields[indices[j]].Name

			if tag := fields[indices[i]].Tag.Get("json"); tag != "" && tag != "-" {
				if tagParts := strings.Split(tag, ","); len(tagParts) > 0 && tagParts[0] != "" {
					nameI = tagParts[0]
				}
			}
			if tag := fields[indices[j]].Tag.Get("json"); tag != "" && tag != "-" {
				if tagParts := strings.Split(tag, ","); len(tagParts) > 0 && tagParts[0] != "" {
					nameJ = tagParts[0]
				}
			}

			return nameI < nameJ
		})

		sortedFields := make([]reflect.StructField, len(fields))
		sortedValues := make([]reflect.Value, len(fieldValues))
		for i, idx := range indices {
			sortedFields[i] = fields[idx]
			sortedValues[i] = fieldValues[idx]
		}
		fields = sortedFields
		fieldValues = sortedValues
	}

	for i, field := range fields {
		fieldValue := fieldValues[i]

		if i > 0 {
			e.buffer.WriteByte(',')
		}

		if e.config.Pretty {
			e.writeIndent()
		}

		jsonTag := field.Tag.Get("json")
		fieldName := field.Name
		if jsonTag != "" && jsonTag != "-" {
			if tagParts := strings.Split(jsonTag, ","); len(tagParts) > 0 && tagParts[0] != "" {
				fieldName = tagParts[0]
			}
		}

		if err := e.encodeString(fieldName); err != nil {
			return err
		}

		e.buffer.WriteByte(':')
		if e.config.Pretty {
			e.buffer.WriteByte(' ')
		}

		if err := e.encodeValue(fieldValue.Interface()); err != nil {
			return err
		}
	}

	e.depth--
	if e.config.Pretty && len(fields) > 0 {
		e.writeIndent()
	}
	e.buffer.WriteByte('}')

	return nil
}

func (e *CustomEncoder) writeIndent() {
	e.buffer.WriteByte('\n')
	e.buffer.WriteString(e.config.Prefix)
	for i := 0; i < e.depth; i++ {
		e.buffer.WriteString(e.config.Indent)
	}
}

func (e *CustomEncoder) isEmpty(v reflect.Value) bool {
	switch v.Kind() {
	case reflect.Array, reflect.Map, reflect.Slice, reflect.String:
		return v.Len() == 0
	case reflect.Bool:
		return !v.Bool()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return v.Int() == 0
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return v.Uint() == 0
	case reflect.Float32, reflect.Float64:
		return v.Float() == 0
	case reflect.Interface, reflect.Ptr:
		return v.IsNil()
	}
	return false
}

// Decoder reads and decodes JSON values from an input stream.
// This type is fully compatible with encoding/json.Decoder.
type Decoder struct {
	r                     io.Reader
	buf                   *bufio.Reader
	processor             *Processor
	useNumber             bool
	disallowUnknownFields bool
	offset                int64
	scanp                 int64 // start of unread data in buf
}

// NewDecoder returns a new decoder that reads from r.
// This function is fully compatible with encoding/json.NewDecoder.
func NewDecoder(r io.Reader) *Decoder {
	return &Decoder{
		r:         r,
		buf:       bufio.NewReader(r),
		processor: getDefaultProcessor(),
	}
}

// Decode reads the next JSON-encoded value from its input and stores it in v.
func (dec *Decoder) Decode(v any) error {
	if v == nil {
		return &InvalidUnmarshalError{Type: nil}
	}

	rv := reflect.ValueOf(v)
	if rv.Kind() != reflect.Ptr || rv.IsNil() {
		return &InvalidUnmarshalError{Type: reflect.TypeOf(v)}
	}

	// Read the next JSON value from the stream
	data, err := dec.readValue()
	if err != nil {
		return err
	}

	// Handle UseNumber directly for compatibility
	if dec.useNumber {
		// Use NumberPreservingDecoder for UseNumber functionality
		decoder := NewNumberPreservingDecoder(true)
		result, err := decoder.DecodeToAny(string(data))
		if err != nil {
			return err
		}

		// Use reflection to assign the result properly
		return dec.assignResult(result, v)
	}

	// Use the processor's Unmarshal method for normal cases
	return dec.processor.Unmarshal(data, v)
}

func (dec *Decoder) assignResult(result any, v any) error {
	rv := reflect.ValueOf(v)
	if rv.Kind() != reflect.Ptr || rv.IsNil() {
		return fmt.Errorf("json: Unmarshal(non-pointer %T)", v)
	}

	rv = rv.Elem()
	resultValue := reflect.ValueOf(result)

	// If the target is any (interface{}), assign directly
	if rv.Kind() == reflect.Interface {
		rv.Set(resultValue)
		return nil
	}

	// For other types, try to assign if compatible
	if resultValue.Type().AssignableTo(rv.Type()) {
		rv.Set(resultValue)
		return nil
	}

	// If not directly assignable, fall back to marshal/unmarshal
	jsonBytes, err := Marshal(result)
	if err != nil {
		return err
	}
	return json.Unmarshal(jsonBytes, v)
}

func (dec *Decoder) UseNumber() {
	dec.useNumber = true
}

func (dec *Decoder) DisallowUnknownFields() {
	dec.disallowUnknownFields = true
}

func (dec *Decoder) Buffered() io.Reader {
	return dec.buf
}

func (dec *Decoder) InputOffset() int64 {
	return dec.offset
}

func (dec *Decoder) More() bool {
	// Peek at the next byte to see if there's more data
	b, err := dec.buf.Peek(1)
	if err != nil {
		return false
	}

	// Skip whitespace
	for len(b) > 0 && isSpace(b[0]) {
		dec.buf.ReadByte()
		b, err = dec.buf.Peek(1)
		if err != nil {
			return false
		}
	}

	if len(b) == 0 {
		return false
	}

	// Check if we're at the end of an array or object
	return b[0] != ']' && b[0] != '}'
}

// Token returns the next JSON token in the input stream.
// At the end of the input stream, Token returns nil, io.EOF.
func (dec *Decoder) Token() (Token, error) {
	// Skip whitespace and separators
	for {
		b, err := dec.buf.ReadByte()
		if err != nil {
			return nil, err
		}
		dec.offset++

		if !isSpace(b) && b != ':' && b != ',' {
			return dec.parseToken(b)
		}
	}
}

func (dec *Decoder) readValue() ([]byte, error) {
	buf := getEncoderBuffer()
	defer putEncoderBuffer(buf)

	depth := 0
	inString := false
	escaped := false

	for {
		b, err := dec.buf.ReadByte()
		if err != nil {
			return nil, err
		}
		dec.offset++

		if !isSpace(b) {
			buf.WriteByte(b)

			switch b {
			case '"':
				inString = true
			case '{', '[':
				depth++
			}
			break
		}
	}

	if depth == 0 && !inString {
		for {
			b, err := dec.buf.ReadByte()
			if err != nil {
				if err == io.EOF {
					break
				}
				return nil, err
			}
			dec.offset++

			if isSpace(b) || b == ',' || b == '}' || b == ']' {
				dec.buf.UnreadByte()
				dec.offset--
				break
			}

			buf.WriteByte(b)
		}
		result := make([]byte, buf.Len())
		copy(result, buf.Bytes())
		return result, nil
	}

	for {
		b, err := dec.buf.ReadByte()
		if err != nil {
			if err == io.EOF && buf.Len() > 0 {
				break
			}
			return nil, err
		}
		dec.offset++
		buf.WriteByte(b)

		if escaped {
			escaped = false
			continue
		}

		if inString {
			if b == '\\' {
				escaped = true
			} else if b == '"' {
				inString = false
				// If we're not in an object/array, string is complete
				if depth == 0 {
					// Return a copy of the buffer data since we're returning it to the pool
					result := make([]byte, buf.Len())
					copy(result, buf.Bytes())
					return result, nil
				}
			}
			continue
		}

		switch b {
		case '"':
			inString = true
		case '{', '[':
			depth++
		case '}', ']':
			depth--
			if depth == 0 {
				// Return a copy of the buffer data since we're returning it to the pool
				result := make([]byte, buf.Len())
				copy(result, buf.Bytes())
				return result, nil
			}
		}
	}

	// Return a copy of the buffer data since we're returning it to the pool
	result := make([]byte, buf.Len())
	copy(result, buf.Bytes())
	return result, nil
}

// parseToken parses a single JSON token starting with the given byte
func (dec *Decoder) parseToken(b byte) (Token, error) {
	switch b {
	case '{':
		return Delim('{'), nil
	case '}':
		return Delim('}'), nil
	case '[':
		return Delim('['), nil
	case ']':
		return Delim(']'), nil
	case '"':
		return dec.parseString()
	case 't', 'f':
		return dec.parseBoolean(b)
	case 'n':
		return dec.parseNull()
	default:
		if isDigit(b) || b == '-' {
			return dec.parseNumber(b)
		}
		return nil, &SyntaxError{
			msg:    fmt.Sprintf("invalid character '%c' looking for beginning of value", b),
			Offset: dec.offset - 1,
		}
	}
}

func (dec *Decoder) parseString() (string, error) {
	buf := getEncoderBuffer()
	defer putEncoderBuffer(buf)

	for {
		b, err := dec.buf.ReadByte()
		if err != nil {
			return "", err
		}
		dec.offset++

		if b == '"' {
			return buf.String(), nil
		}

		if b == '\\' {
			next, err := dec.buf.ReadByte()
			if err != nil {
				return "", err
			}
			dec.offset++

			switch next {
			case '"', '\\', '/':
				buf.WriteByte(next)
			case 'b':
				buf.WriteByte('\b')
			case 'f':
				buf.WriteByte('\f')
			case 'n':
				buf.WriteByte('\n')
			case 'r':
				buf.WriteByte('\r')
			case 't':
				buf.WriteByte('\t')
			case 'u':
				var hex [4]byte
				for i := 0; i < 4; i++ {
					hex[i], err = dec.buf.ReadByte()
					if err != nil {
						return "", err
					}
					dec.offset++
				}

				code, err := strconv.ParseUint(string(hex[:]), 16, 16)
				if err != nil {
					return "", err
				}
				buf.WriteRune(rune(code))
			default:
				return "", &SyntaxError{
					msg:    fmt.Sprintf("invalid escape sequence '\\%c'", next),
					Offset: dec.offset - 2,
				}
			}
		} else {
			buf.WriteByte(b)
		}
	}
}

func (dec *Decoder) parseBoolean(first byte) (bool, error) {
	if first == 't' {
		expected := "rue"
		for i, expected_char := range expected {
			b, err := dec.buf.ReadByte()
			if err != nil {
				return false, err
			}
			dec.offset++
			if b != byte(expected_char) {
				return false, &SyntaxError{
					msg:    fmt.Sprintf("invalid character '%c' in literal true (expecting '%c')", b, expected_char),
					Offset: dec.offset - int64(i) - 2,
				}
			}
		}
		return true, nil
	}

	expected := "alse"
	for i, expected_char := range expected {
		b, err := dec.buf.ReadByte()
		if err != nil {
			return false, err
		}
		dec.offset++
		if b != byte(expected_char) {
			return false, &SyntaxError{
				msg:    fmt.Sprintf("invalid character '%c' in literal false (expecting '%c')", b, expected_char),
				Offset: dec.offset - int64(i) - 2,
			}
		}
	}
	return false, nil
}

func (dec *Decoder) parseNull() (any, error) {
	expected := "ull"
	for i, expectedChar := range expected {
		b, err := dec.buf.ReadByte()
		if err != nil {
			return nil, err
		}
		dec.offset++
		if b != byte(expectedChar) {
			return nil, &SyntaxError{
				msg:    fmt.Sprintf("invalid character '%c' in literal null (expecting '%c')", b, expectedChar),
				Offset: dec.offset - int64(i) - 2,
			}
		}
	}
	return nil, nil
}

func (dec *Decoder) parseNumber(first byte) (any, error) {
	buf := getEncoderBuffer()
	defer putEncoderBuffer(buf)
	buf.WriteByte(first)

	for {
		b, err := dec.buf.Peek(1)
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}

		if !isDigit(b[0]) && b[0] != '.' && b[0] != 'e' && b[0] != 'E' && b[0] != '+' && b[0] != '-' {
			break
		}

		actual, _ := dec.buf.ReadByte()
		dec.offset++
		buf.WriteByte(actual)
	}

	numStr := buf.String()

	if dec.useNumber {
		return Number(numStr), nil
	}

	if !strings.Contains(numStr, ".") && !strings.Contains(numStr, "e") && !strings.Contains(numStr, "E") {
		if val, err := strconv.ParseInt(numStr, 10, 64); err == nil {
			return val, nil
		}
	}

	// Parse as float64
	val, err := strconv.ParseFloat(numStr, 64)
	if err != nil {
		return nil, &SyntaxError{
			msg:    fmt.Sprintf("invalid number: %s", numStr),
			Offset: dec.offset - int64(len(numStr)),
		}
	}

	return val, nil
}

// Encoder writes JSON values to an output stream.
// This type is fully compatible with encoding/json.Encoder.
type Encoder struct {
	w          io.Writer
	processor  *Processor
	escapeHTML bool
	indent     string
	prefix     string
}

// NewEncoder returns a new encoder that writes to w.
// This function is fully compatible with encoding/json.NewEncoder.
func NewEncoder(w io.Writer) *Encoder {
	return &Encoder{
		w:          w,
		processor:  getDefaultProcessor(),
		escapeHTML: true, // Default behavior matches encoding/json
	}
}

// Encode writes the JSON encoding of v to the stream,
// followed by a newline character.
//
// See the documentation for Marshal for details about the
// conversion of Go values to JSON.
func (enc *Encoder) Encode(v any) error {
	// Create encoding config based on encoder settings
	config := DefaultEncodeConfig()
	config.EscapeHTML = enc.escapeHTML

	if enc.indent != "" || enc.prefix != "" {
		config.Pretty = true
		config.Indent = enc.indent
		config.Prefix = enc.prefix
	}

	// Encode the value
	jsonStr, err := enc.processor.EncodeWithConfig(v, config)
	if err != nil {
		return err
	}

	// Write to the output stream with a newline
	_, err = enc.w.Write([]byte(jsonStr + "\n"))
	return err
}

// SetEscapeHTML specifies whether problematic HTML characters
// should be escaped inside JSON quoted strings.
// The default behavior is to escape &, <, and > to \u0026, \u003c, and \u003e
// to avoid certain safety problems that can arise when embedding JSON in HTML.
//
// In non-HTML settings where the escaping interferes with the readability
// of the output, SetEscapeHTML(false) disables this behavior.
func (enc *Encoder) SetEscapeHTML(on bool) {
	enc.escapeHTML = on
}

// SetIndent instructs the encoder to format each subsequent encoded
// value as if indented by the package-level function Indent(dst, src, prefix, indent).
// Calling SetIndent("", "") disables indentation.
func (enc *Encoder) SetIndent(prefix, indent string) {
	enc.prefix = prefix
	enc.indent = indent
}

// Token holds a value of one of these types:
//
//	Delim, for the four JSON delimiters [ ] { }
//	bool, for JSON booleans
//	float64, for JSON numbers
//	Number, for JSON numbers
//	string, for JSON string literals
//	nil, for JSON null
type Token any

// Delim is a JSON delimiter.
type Delim rune

func (d Delim) String() string {
	return string(d)
}

// Number represents a JSON number literal.
type Number string

// String returns the literal text of the number.
func (n Number) String() string { return string(n) }

// Float64 returns the number as a float64.
func (n Number) Float64() (float64, error) {
	return strconv.ParseFloat(string(n), 64)
}

// Int64 returns the number as an int64.
func (n Number) Int64() (int64, error) {
	return strconv.ParseInt(string(n), 10, 64)
}

// isSpace reports whether the character is a JSON whitespace character.
func isSpace(c byte) bool {
	return c == ' ' || c == '\t' || c == '\r' || c == '\n'
}

// isDigit reports whether the character is a digit.
func isDigit(c byte) bool {
	return '0' <= c && c <= '9'
}

// ValidateSchema validates JSON data against a schema
func (p *Processor) ValidateSchema(jsonStr string, schema *Schema, opts ...*ProcessorOptions) ([]ValidationError, error) {
	if err := p.checkClosed(); err != nil {
		return nil, err
	}

	_, err := p.prepareOptions(opts...)
	if err != nil {
		return nil, err
	}

	if err := p.validateInput(jsonStr); err != nil {
		return nil, err
	}

	if schema == nil {
		return nil, &JsonsError{
			Op:      "validate_schema",
			Message: "schema cannot be nil",
			Err:     ErrOperationFailed,
		}
	}

	// Parse JSON

	var data any
	err = p.Parse(jsonStr, &data, opts...)
	if err != nil {
		return nil, err
	}

	// Valid against schema
	var errors []ValidationError
	p.validateValue(data, schema, "", &errors)

	return errors, nil
}

// validateValue validates a value against a schema with improved performance
func (p *Processor) validateValue(value any, schema *Schema, path string, errors *[]ValidationError) {
	if schema == nil {
		return
	}

	// Constant value validation
	if schema.Const != nil {
		if !p.valuesEqual(value, schema.Const) {
			*errors = append(*errors, ValidationError{
				Path:    path,
				Message: fmt.Sprintf("value must be constant: %v", schema.Const),
			})
			return
		}
	}

	// Enum validation
	if len(schema.Enum) > 0 {
		found := false
		for _, enumValue := range schema.Enum {
			if p.valuesEqual(value, enumValue) {
				found = true
				break
			}
		}
		if !found {
			*errors = append(*errors, ValidationError{
				Path:    path,
				Message: fmt.Sprintf("value '%v' is not in allowed enum values: %v", value, schema.Enum),
			})
			return
		}
	}

	// Type validation
	if schema.Type != "" {
		if !p.validateType(value, schema.Type) {
			*errors = append(*errors, ValidationError{
				Path:    path,
				Message: fmt.Sprintf("expected type %s, got %T", schema.Type, value),
			})
			return
		}
	}

	// Type-specific validations using switch for better performance
	switch schema.Type {
	case "object":
		if obj, ok := value.(map[string]any); ok {
			p.validateObject(obj, schema, path, errors)
		}
	case "array":
		if arr, ok := value.([]any); ok {
			p.validateArray(arr, schema, path, errors)
		}
	case "string":
		if str, ok := value.(string); ok {
			p.validateString(str, schema, path, errors)
		}
	case "number":
		p.validateNumber(value, schema, path, errors)
	}
}

// validateType checks if a value matches the expected type
func (p *Processor) validateType(value any, expectedType string) bool {
	switch expectedType {
	case "object":
		_, ok := value.(map[string]any)
		return ok
	case "array":
		_, ok := value.([]any)
		return ok
	case "string":
		_, ok := value.(string)
		return ok
	case "number":
		switch value.(type) {
		case int, int32, int64, float32, float64:
			return true
		}
		return false
	case "boolean":
		_, ok := value.(bool)
		return ok
	case "null":
		return value == nil
	}
	return false
}

// validateObject validates an object against a schema with type safety
func (p *Processor) validateObject(obj map[string]any, schema *Schema, path string, errors *[]ValidationError) {
	// Required properties validation
	for _, required := range schema.Required {
		if _, exists := obj[required]; !exists {
			*errors = append(*errors, ValidationError{
				Path:    p.joinPath(path, required),
				Message: fmt.Sprintf("required property '%s' is missing", required),
			})
		}
	}

	// Valid properties
	for key, val := range obj {
		if propSchema, exists := schema.Properties[key]; exists {
			p.validateValue(val, propSchema, p.joinPath(path, key), errors)
		} else if !schema.AdditionalProperties {
			*errors = append(*errors, ValidationError{
				Path:    p.joinPath(path, key),
				Message: fmt.Sprintf("additional property '%s' is not allowed", key),
			})
		}
	}
}

// validateArray validates an array against a schema with type safety
func (p *Processor) validateArray(arr []any, schema *Schema, path string, errors *[]ValidationError) {
	arrLen := len(arr)

	// Array length validation
	if schema.HasMinItems() && arrLen < schema.MinItems {
		*errors = append(*errors, ValidationError{
			Path:    path,
			Message: fmt.Sprintf("array length %d is less than minimum %d", arrLen, schema.MinItems),
		})
	}

	if schema.HasMaxItems() && arrLen > schema.MaxItems {
		*errors = append(*errors, ValidationError{
			Path:    path,
			Message: fmt.Sprintf("array length %d exceeds maximum %d", arrLen, schema.MaxItems),
		})
	}

	// Unique items validation
	if schema.UniqueItems {
		seen := make(map[string]bool)
		for i, item := range arr {
			itemStr := fmt.Sprintf("%v", item)
			if seen[itemStr] {
				*errors = append(*errors, ValidationError{
					Path:    fmt.Sprintf("%s[%d]", path, i),
					Message: fmt.Sprintf("duplicate item found: %v", item),
				})
			}
			seen[itemStr] = true
		}
	}

	// Validate items
	if schema.Items != nil {
		for i, item := range arr {
			itemPath := fmt.Sprintf("%s[%d]", path, i)
			p.validateValue(item, schema.Items, itemPath, errors)
		}
	}
}

// validateString validates a string against a schema with type safety
func (p *Processor) validateString(str string, schema *Schema, path string, errors *[]ValidationError) {
	// Length validation
	strLen := len(str)
	if schema.HasMinLength() && strLen < schema.MinLength {
		*errors = append(*errors, ValidationError{
			Path:    path,
			Message: fmt.Sprintf("string length %d is less than minimum %d", strLen, schema.MinLength),
		})
	}

	if schema.HasMaxLength() && strLen > schema.MaxLength {
		*errors = append(*errors, ValidationError{
			Path:    path,
			Message: fmt.Sprintf("string length %d exceeds maximum %d", strLen, schema.MaxLength),
		})
	}

	// Pattern validation (regular expression)
	if schema.Pattern != "" {
		matched, err := regexp.MatchString(schema.Pattern, str)
		if err != nil {
			*errors = append(*errors, ValidationError{
				Path:    path,
				Message: fmt.Sprintf("invalid pattern '%s': %v", schema.Pattern, err),
			})
		} else if !matched {
			*errors = append(*errors, ValidationError{
				Path:    path,
				Message: fmt.Sprintf("string '%s' does not match pattern '%s'", str, schema.Pattern),
			})
		}
	}

	// Format validation
	if schema.Format != "" {
		if err := p.validateStringFormat(str, schema.Format, path, errors); err != nil {
			*errors = append(*errors, ValidationError{
				Path:    path,
				Message: fmt.Sprintf("format validation failed: %v", err),
			})
		}
	}
}

// validateNumber validates a number against a schema
func (p *Processor) validateNumber(value any, schema *Schema, path string, errors *[]ValidationError) {
	var num float64
	switch v := value.(type) {
	case int:
		num = float64(v)
	case int32:
		num = float64(v)
	case int64:
		num = float64(v)
	case float32:
		num = float64(v)
	case float64:
		num = v
	default:
		return
	}

	// Range validation - only validate if constraints are explicitly set
	if schema.HasMinimum() {
		if schema.ExclusiveMinimum {
			if num <= schema.Minimum {
				*errors = append(*errors, ValidationError{
					Path:    path,
					Message: fmt.Sprintf("number %g must be greater than %g (exclusive)", num, schema.Minimum),
				})
			}
		} else {
			if num < schema.Minimum {
				*errors = append(*errors, ValidationError{
					Path:    path,
					Message: fmt.Sprintf("number %g is less than minimum %g", num, schema.Minimum),
				})
			}
		}
	}

	if schema.HasMaximum() {
		if schema.ExclusiveMaximum {
			if num >= schema.Maximum {
				*errors = append(*errors, ValidationError{
					Path:    path,
					Message: fmt.Sprintf("number %g must be less than %g (exclusive)", num, schema.Maximum),
				})
			}
		} else {
			if num > schema.Maximum {
				*errors = append(*errors, ValidationError{
					Path:    path,
					Message: fmt.Sprintf("number %g exceeds maximum %g", num, schema.Maximum),
				})
			}
		}
	}

	// Multiple of validation
	if schema.MultipleOf > 0 {
		if remainder := num / schema.MultipleOf; remainder != float64(int(remainder)) {
			*errors = append(*errors, ValidationError{
				Path:    path,
				Message: fmt.Sprintf("number %g is not a multiple of %g", num, schema.MultipleOf),
			})
		}
	}
}

// validateStringFormat validates string format (email, date, etc.)
func (p *Processor) validateStringFormat(str, format, path string, errors *[]ValidationError) error {
	switch format {
	case "email":
		return p.validateEmailFormat(str, path, errors)
	case "date":
		return p.validateDateFormat(str, path, errors)
	case "date-time":
		return p.validateDateTimeFormat(str, path, errors)
	case "time":
		return p.validateTimeFormat(str, path, errors)
	case "uri":
		return p.validateURIFormat(str, path, errors)
	case "uuid":
		return p.validateUUIDFormat(str, path, errors)
	case "ipv4":
		return p.validateIPv4Format(str, path, errors)
	case "ipv6":
		return p.validateIPv6Format(str, path, errors)
	default:
		// Unknown format - log warning but don't fail validation
		return nil
	}
}

// validateEmailFormat validates email format with improved security
// Prevents consecutive dots, limits length, and validates proper structure
func (p *Processor) validateEmailFormat(email, path string, errors *[]ValidationError) error {
	// Length validation to prevent DoS
	if len(email) > 254 { // RFC 5321 limit
		*errors = append(*errors, ValidationError{
			Path:    path,
			Message: fmt.Sprintf("'%s' exceeds maximum email length of 254 characters", email),
		})
		return nil
	}

	// Split into local and domain parts
	atIndex := strings.LastIndex(email, "@")
	if atIndex <= 0 || atIndex == len(email)-1 {
		*errors = append(*errors, ValidationError{
			Path:    path,
			Message: fmt.Sprintf("'%s' is not a valid email format", email),
		})
		return nil
	}

	localPart := email[:atIndex]
	domainPart := email[atIndex+1:]

	// Validate local part (max 64 chars as per RFC 5321)
	if len(localPart) > 64 || len(localPart) == 0 {
		*errors = append(*errors, ValidationError{
			Path:    path,
			Message: fmt.Sprintf("'%s' has invalid local part in email address", email),
		})
		return nil
	}

	// Check for consecutive dots or invalid characters
	if strings.Contains(localPart, "..") || strings.Contains(localPart, ".@") ||
		strings.HasPrefix(localPart, ".") || strings.HasSuffix(localPart, ".") {
		*errors = append(*errors, ValidationError{
			Path:    path,
			Message: fmt.Sprintf("'%s' has invalid local part in email address", email),
		})
		return nil
	}

	// Validate domain part
	if len(domainPart) > 253 || len(domainPart) == 0 {
		*errors = append(*errors, ValidationError{
			Path:    path,
			Message: fmt.Sprintf("'%s' has invalid domain in email address", email),
		})
		return nil
	}

	// Check for consecutive dots in domain
	if strings.Contains(domainPart, "..") || strings.HasPrefix(domainPart, ".") ||
		strings.HasSuffix(domainPart, ".") {
		*errors = append(*errors, ValidationError{
			Path:    path,
			Message: fmt.Sprintf("'%s' has invalid domain in email address", email),
		})
		return nil
	}

	// Validate domain has at least one dot and TLD is at least 2 chars
	dotCount := strings.Count(domainPart, ".")
	if dotCount < 1 {
		*errors = append(*errors, ValidationError{
			Path:    path,
			Message: fmt.Sprintf("'%s' has invalid domain in email address", email),
		})
		return nil
	}

	// Check TLD length
	lastDot := strings.LastIndex(domainPart, ".")
	tld := domainPart[lastDot+1:]
	if len(tld) < 2 || len(tld) > 63 {
		*errors = append(*errors, ValidationError{
			Path:    path,
			Message: fmt.Sprintf("'%s' has invalid TLD in email address", email),
		})
		return nil
	}

	// Basic character validation for local and domain parts
	localRegex := regexp.MustCompile(`^[a-zA-Z0-9._%+-]+$`)
	domainRegex := regexp.MustCompile(`^[a-zA-Z0-9.-]+$`)

	if !localRegex.MatchString(localPart) || !domainRegex.MatchString(domainPart) {
		*errors = append(*errors, ValidationError{
			Path:    path,
			Message: fmt.Sprintf("'%s' contains invalid characters in email address", email),
		})
		return nil
	}

	return nil
}

// validateDateFormat validates date format (YYYY-MM-DD)
func (p *Processor) validateDateFormat(date, path string, errors *[]ValidationError) error {
	_, err := time.Parse("2006-01-02", date)
	if err != nil {
		*errors = append(*errors, ValidationError{
			Path:    path,
			Message: fmt.Sprintf("'%s' is not a valid date format (expected YYYY-MM-DD)", date),
		})
	}
	return nil
}

// validateDateTimeFormat validates date-time format (RFC3339)
func (p *Processor) validateDateTimeFormat(datetime, path string, errors *[]ValidationError) error {
	_, err := time.Parse(time.RFC3339, datetime)
	if err != nil {
		*errors = append(*errors, ValidationError{
			Path:    path,
			Message: fmt.Sprintf("'%s' is not a valid date-time format (expected RFC3339)", datetime),
		})
	}
	return nil
}

// validateTimeFormat validates time format (HH:MM:SS)
func (p *Processor) validateTimeFormat(timeStr, path string, errors *[]ValidationError) error {
	_, err := time.Parse("15:04:05", timeStr)
	if err != nil {
		*errors = append(*errors, ValidationError{
			Path:    path,
			Message: fmt.Sprintf("'%s' is not a valid time format (expected HH:MM:SS)", timeStr),
		})
	}
	return nil
}

// validateURIFormat validates URI format
func (p *Processor) validateURIFormat(uri, path string, errors *[]ValidationError) error {
	// Simple URI validation - check for scheme
	if !strings.Contains(uri, "://") {
		*errors = append(*errors, ValidationError{
			Path:    path,
			Message: fmt.Sprintf("'%s' is not a valid URI format", uri),
		})
	}
	return nil
}

// validateUUIDFormat validates UUID format
func (p *Processor) validateUUIDFormat(uuid, path string, errors *[]ValidationError) error {
	// UUID v4 regex pattern
	uuidRegex := `^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`
	matched, err := regexp.MatchString(uuidRegex, uuid)
	if err != nil {
		return err
	}
	if !matched {
		*errors = append(*errors, ValidationError{
			Path:    path,
			Message: fmt.Sprintf("'%s' is not a valid UUID format", uuid),
		})
	}
	return nil
}

// validateIPv4Format validates IPv4 format
func (p *Processor) validateIPv4Format(ip, path string, errors *[]ValidationError) error {
	parts := strings.Split(ip, ".")
	if len(parts) != 4 {
		*errors = append(*errors, ValidationError{
			Path:    path,
			Message: fmt.Sprintf("'%s' is not a valid IPv4 format", ip),
		})
		return nil
	}

	for _, part := range parts {
		num, err := strconv.Atoi(part)
		if err != nil || num < 0 || num > 255 {
			*errors = append(*errors, ValidationError{
				Path:    path,
				Message: fmt.Sprintf("'%s' is not a valid IPv4 format", ip),
			})
			return nil
		}
	}
	return nil
}

// validateIPv6Format validates IPv6 format
func (p *Processor) validateIPv6Format(ip, path string, errors *[]ValidationError) error {
	// Simple IPv6 validation - check for colons and hex characters
	if !strings.Contains(ip, ":") {
		*errors = append(*errors, ValidationError{
			Path:    path,
			Message: fmt.Sprintf("'%s' is not a valid IPv6 format", ip),
		})
		return nil
	}

	// More detailed validation could be added here
	ipv6Regex := `^([0-9a-fA-F]{0,4}:){2,7}[0-9a-fA-F]{0,4}$`
	matched, err := regexp.MatchString(ipv6Regex, ip)
	if err != nil {
		return err
	}
	if !matched {
		*errors = append(*errors, ValidationError{
			Path:    path,
			Message: fmt.Sprintf("'%s' is not a valid IPv6 format", ip),
		})
	}
	return nil
}

// valuesEqual compares two values for equality
func (p *Processor) valuesEqual(a, b any) bool {
	// Handle nil cases
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}

	// Direct comparison for basic types
	if a == b {
		return true
	}

	// Handle numeric type conversions
	switch va := a.(type) {
	case int:
		switch vb := b.(type) {
		case int:
			return va == vb
		case int32:
			return int32(va) == vb
		case int64:
			return int64(va) == vb
		case float32:
			return float32(va) == vb
		case float64:
			return float64(va) == vb
		}
	case float64:
		switch vb := b.(type) {
		case int:
			return va == float64(vb)
		case int32:
			return va == float64(vb)
		case int64:
			return va == float64(vb)
		case float32:
			return va == float64(vb)
		case float64:
			return va == vb
		}
	}

	return false
}

// joinPath joins path segments
func (p *Processor) joinPath(parent, child string) string {
	if parent == "" {
		return child
	}
	return parent + "." + child
}
