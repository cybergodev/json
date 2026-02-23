package json

import (
	"io"
)

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
