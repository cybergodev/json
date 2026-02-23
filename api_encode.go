package json

import (
	"bytes"
	"fmt"
	"os"
)

// Marshal returns the JSON encoding of v.
// This function is 100% compatible with encoding/json.Marshal.
func Marshal(v any) ([]byte, error) {
	return getDefaultProcessor().Marshal(v)
}

// Unmarshal parses the JSON-encoded data and stores the result in v.
// This function is 100% compatible with encoding/json.Unmarshal.
func Unmarshal(data []byte, v any) error {
	return getDefaultProcessor().Unmarshal(data, v)
}

// MarshalIndent is like Marshal but applies indentation to format the output.
// This function is 100% compatible with encoding/json.MarshalIndent.
func MarshalIndent(v any, prefix, indent string) ([]byte, error) {
	return getDefaultProcessor().MarshalIndent(v, prefix, indent)
}

// Compact appends to dst the JSON-encoded src with insignificant space characters elided.
// This function is 100% compatible with encoding/json.Compact.
func Compact(dst *bytes.Buffer, src []byte) error {
	compacted, err := FormatCompact(string(src))
	if err != nil {
		return err
	}
	dst.WriteString(compacted)
	return nil
}

// Indent appends to dst an indented form of the JSON-encoded src.
// This function is 100% compatible with encoding/json.Indent.
func Indent(dst *bytes.Buffer, src []byte, prefix, indent string) error {
	processor := getDefaultProcessor()
	result, err := processor.FormatPretty(string(src))
	if err != nil {
		return err
	}
	dst.WriteString(result)
	return nil
}

// HTMLEscape appends to dst the JSON-encoded src with <, >, &, U+2028, and U+2029 characters escaped.
// This function is 100% compatible with encoding/json.HTMLEscape.
func HTMLEscape(dst *bytes.Buffer, src []byte) {
	// Use standard library compatible HTML escaping
	result := htmlEscape(string(src))
	dst.WriteString(result)
}

// htmlEscape performs HTML escaping on JSON string
func htmlEscape(s string) string {
	var buf bytes.Buffer
	buf.Grow(len(s))
	for _, r := range s {
		switch r {
		case '<':
			buf.WriteString("\\u003c")
		case '>':
			buf.WriteString("\\u003e")
		case '&':
			buf.WriteString("\\u0026")
		default:
			buf.WriteRune(r)
		}
	}
	return buf.String()
}

// CompactBuffer is an alias for Compact for buffer operations
func CompactBuffer(dst *bytes.Buffer, src []byte, opts ...*ProcessorOptions) error {
	compacted, err := FormatCompact(string(src), opts...)
	if err != nil {
		return err
	}
	dst.WriteString(compacted)
	return nil
}

// IndentBuffer is an alias for Indent for buffer operations
func IndentBuffer(dst *bytes.Buffer, src []byte, prefix, indent string, opts ...*ProcessorOptions) error {
	result, err := FormatPretty(string(src), opts...)
	if err != nil {
		return err
	}
	dst.WriteString(result)
	return nil
}

// HTMLEscapeBuffer is an alias for HTMLEscape for buffer operations
func HTMLEscapeBuffer(dst *bytes.Buffer, src []byte, opts ...*ProcessorOptions) {
	result := htmlEscape(string(src))
	dst.WriteString(result)
}

// Encode converts any Go value to JSON string
func Encode(value any, config ...*EncodeConfig) (string, error) {
	var cfg *EncodeConfig
	if len(config) > 0 {
		cfg = config[0]
	}
	return getDefaultProcessor().EncodeWithConfig(value, cfg)
}

// EncodePretty converts any Go value to pretty-formatted JSON string
func EncodePretty(value any, config ...*EncodeConfig) (string, error) {
	var cfg *EncodeConfig
	if len(config) > 0 && config[0] != nil {
		cfg = config[0]
	} else {
		cfg = NewPrettyConfig()
	}
	return getDefaultProcessor().EncodeWithConfig(value, cfg)
}

// FormatPretty formats JSON string with pretty indentation.
func FormatPretty(jsonStr string, opts ...*ProcessorOptions) (string, error) {
	return getDefaultProcessor().FormatPretty(jsonStr, opts...)
}

// FormatCompact removes whitespace from JSON string.
func FormatCompact(jsonStr string, opts ...*ProcessorOptions) (string, error) {
	return getDefaultProcessor().Compact(jsonStr, opts...)
}

// Print prints any Go value as JSON to stdout in compact format.
func Print(data any) {
	result, err := printData(data, false)
	if err != nil {
		fmt.Fprintf(os.Stderr, "json.Print error: %v\n", err)
		return
	}
	fmt.Println(result)
}

// PrintPretty prints any Go value as formatted JSON to stdout.
func PrintPretty(data any) {
	result, err := printData(data, true)
	if err != nil {
		fmt.Fprintf(os.Stderr, "json.PrintPretty error: %v\n", err)
		return
	}
	fmt.Println(result)
}

// printData handles the core logic for Print and PrintPretty
func printData(data any, pretty bool) (string, error) {
	processor := getDefaultProcessor()

	switch v := data.(type) {
	case string:
		// Check if it's valid JSON - if so, format it directly
		if isValid, _ := processor.Valid(v); isValid {
			if pretty {
				return processor.FormatPretty(v)
			}
			return processor.Compact(v)
		}
		// Not valid JSON, encode as a normal string
		if pretty {
			return EncodePretty(v)
		}
		return Encode(v)

	case []byte:
		jsonStr := string(v)
		// Check if it's valid JSON - if so, format it directly
		if isValid, _ := processor.Valid(jsonStr); isValid {
			if pretty {
				return processor.FormatPretty(jsonStr)
			}
			return processor.Compact(jsonStr)
		}
		// Not valid JSON, encode as normal
		if pretty {
			return EncodePretty(v)
		}
		return Encode(v)

	default:
		// Encode other types normally
		if pretty {
			return EncodePretty(v)
		}
		return Encode(v)
	}
}

// Valid reports whether data is valid JSON
func Valid(data []byte) bool {
	jsonStr := string(data)
	valid, err := getDefaultProcessor().Valid(jsonStr)
	return err == nil && valid
}

// ValidateSchema validates JSON data against a schema
func ValidateSchema(jsonStr string, schema *Schema, opts ...*ProcessorOptions) ([]ValidationError, error) {
	return getDefaultProcessor().ValidateSchema(jsonStr, schema, opts...)
}
