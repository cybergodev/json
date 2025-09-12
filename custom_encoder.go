package json

import (
	"bytes"
	"encoding"
	"encoding/json"
	"fmt"
	"log"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

// Buffer pools for custom encoder memory optimization
var (
	encoderBufferPool = sync.Pool{
		New: func() interface{} {
			return &bytes.Buffer{}
		},
	}
)

// getEncoderBuffer gets a bytes.Buffer from the encoder pool
func getEncoderBuffer() *bytes.Buffer {
	buf := encoderBufferPool.Get().(*bytes.Buffer)
	buf.Reset()
	return buf
}

// putEncoderBuffer returns a bytes.Buffer to the encoder pool
func putEncoderBuffer(buf *bytes.Buffer) {
	if buf != nil && buf.Cap() <= 8192 { // Prevent memory bloat
		encoderBufferPool.Put(buf)
	}
}

// CustomEncoder provides advanced JSON encoding with configurable options
type CustomEncoder struct {
	config *EncodeConfig
	buffer *bytes.Buffer
	depth  int

	// Pre-allocated buffers for better performance
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
	// Check both the original value and the dereferenced value
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

// encodeFloat encodes float values with proper precision handling
func (e *CustomEncoder) encodeFloat(f float64, bits int) error {
	// If FloatPrecision is set (>= 0), use it regardless of PreserveNumbers setting
	if e.config.FloatPrecision >= 0 {
		formatted := strconv.FormatFloat(f, 'f', e.config.FloatPrecision, bits)
		e.buffer.WriteString(formatted)
		return nil
	}

	// Use 'f' format for reasonable range to avoid scientific notation
	// This helps maintain consistency in number representation
	if f >= -1e15 && f <= 1e15 {
		formatted := strconv.FormatFloat(f, 'f', -1, bits)
		e.buffer.WriteString(formatted)
	} else {
		// Use 'g' format for very large/small numbers
		formatted := strconv.FormatFloat(f, 'g', -1, bits)
		e.buffer.WriteString(formatted)
	}

	return nil
}

// encodeString encodes a string with custom escaping options and improved performance
func (e *CustomEncoder) encodeString(s string) error {
	e.buffer.WriteByte('"')

	if e.config.DisableEscaping {
		// Minimal escaping - only escape quotes and backslashes
		// Use byte-level operations for ASCII characters for better performance
		for i := 0; i < len(s); i++ {
			b := s[i]
			switch b {
			case '"':
				e.buffer.WriteString(`\"`)
			case '\\':
				e.buffer.WriteString(`\\`)
			default:
				if b < 0x80 {
					// ASCII character, write directly
					e.buffer.WriteByte(b)
				} else {
					// Non-ASCII, decode rune and write
					r, size := utf8.DecodeRuneInString(s[i:])
					e.buffer.WriteRune(r)
					i += size - 1 // Adjust for loop increment
				}
			}
		}
	} else {
		// Standard escaping with custom options
		for _, r := range s {
			if err := e.escapeRune(r); err != nil {
				return err
			}
		}
	}

	e.buffer.WriteByte('"')
	return nil
}

// escapeRune escapes a single rune according to options
func (e *CustomEncoder) escapeRune(r rune) error {
	// Check custom escapes first
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
			// Control characters
			e.buffer.WriteString(fmt.Sprintf(`\u%04x`, r))
		} else if e.config.EscapeHTML && (r == '<' || r == '>' || r == '&') {
			// HTML escaping
			e.buffer.WriteString(fmt.Sprintf(`\u%04x`, r))
		} else if e.config.EscapeUnicode && r > 0x7F {
			// Unicode escaping
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

// encodeArray encodes arrays and slices
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

// encodeMap encodes maps
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

		// Skip empty values if OmitEmpty is enabled
		if e.config.OmitEmpty && e.isEmpty(value) {
			continue
		}

		// Skip null values if IncludeNulls is disabled
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

		// Encode key
		if err := e.encodeString(key.String()); err != nil {
			return err
		}

		e.buffer.WriteByte(':')
		if e.config.Pretty {
			e.buffer.WriteByte(' ')
		}

		// Encode value
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

// encodeStruct encodes structs with custom field handling
func (e *CustomEncoder) encodeStruct(v reflect.Value) error {
	// If we need custom field handling (OmitEmpty, IncludeNulls), implement it manually
	if e.config.OmitEmpty || !e.config.IncludeNulls || e.config.SortKeys {
		return e.encodeStructCustom(v)
	}

	// For simple cases, use standard JSON encoding with Pretty support
	if e.config.Pretty {
		data, err := json.MarshalIndent(v.Interface(), e.config.Prefix, e.config.Indent)
		if err != nil {
			return err
		}
		e.buffer.Write(data)
		return nil
	}

	// For compact formatting, use standard JSON encoding
	data, err := json.Marshal(v.Interface())
	if err != nil {
		return err
	}
	e.buffer.Write(data)
	return nil
}

// encodeStructCustom encodes structs with custom field handling for OmitEmpty and IncludeNulls
func (e *CustomEncoder) encodeStructCustom(v reflect.Value) error {
	e.buffer.WriteByte('{')
	e.depth++

	t := v.Type()
	var fields []reflect.StructField
	var fieldValues []reflect.Value

	// Collect all fields and their values
	for i := 0; i < v.NumField(); i++ {
		field := t.Field(i)
		fieldValue := v.Field(i)

		// Skip unexported fields
		if !field.IsExported() {
			continue
		}

		// Get JSON tag name
		jsonTag := field.Tag.Get("json")
		if jsonTag == "-" {
			continue // Skip fields with json:"-"
		}

		// Parse JSON tag options
		tagParts := strings.Split(jsonTag, ",")

		// Check for omitempty tag
		hasOmitEmpty := false
		for _, part := range tagParts[1:] {
			if part == "omitempty" {
				hasOmitEmpty = true
				break
			}
		}

		// Apply field filtering logic
		shouldSkip := false

		// Skip empty values if OmitEmpty is enabled globally or via tag
		if (e.config.OmitEmpty || hasOmitEmpty) && e.isEmpty(fieldValue) {
			shouldSkip = true
		}

		// Skip null values if IncludeNulls is disabled
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

	// Sort fields if requested
	if e.config.SortKeys {
		// Create a slice of indices for sorting
		indices := make([]int, len(fields))
		for i := range indices {
			indices[i] = i
		}

		sort.Slice(indices, func(i, j int) bool {
			nameI := fields[indices[i]].Name
			nameJ := fields[indices[j]].Name

			// Use JSON tag name if available
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

		// Reorder fields and values according to sorted indices
		sortedFields := make([]reflect.StructField, len(fields))
		sortedValues := make([]reflect.Value, len(fieldValues))
		for i, idx := range indices {
			sortedFields[i] = fields[idx]
			sortedValues[i] = fieldValues[idx]
		}
		fields = sortedFields
		fieldValues = sortedValues
	}

	// Encode fields
	for i, field := range fields {
		fieldValue := fieldValues[i]

		if i > 0 {
			e.buffer.WriteByte(',')
		}

		if e.config.Pretty {
			e.writeIndent()
		}

		// Get field name from JSON tag or struct field name
		jsonTag := field.Tag.Get("json")
		fieldName := field.Name
		if jsonTag != "" && jsonTag != "-" {
			if tagParts := strings.Split(jsonTag, ","); len(tagParts) > 0 && tagParts[0] != "" {
				fieldName = tagParts[0]
			}
		}

		// Encode field name
		if err := e.encodeString(fieldName); err != nil {
			return err
		}

		e.buffer.WriteByte(':')
		if e.config.Pretty {
			e.buffer.WriteByte(' ')
		}

		// Encode field value
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

// writeIndent writes indentation for pretty printing
func (e *CustomEncoder) writeIndent() {
	e.buffer.WriteByte('\n')
	e.buffer.WriteString(e.config.Prefix)
	for i := 0; i < e.depth; i++ {
		e.buffer.WriteString(e.config.Indent)
	}
}

// isEmpty checks if a value is considered empty for OmitEmpty option
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
	default:
		log.Println("Unmatched type:", v.Kind())
	}
	return false
}
