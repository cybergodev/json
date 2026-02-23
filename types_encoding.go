package json

// EncodeConfig provides advanced encoding configuration (for complex use cases)
type EncodeConfig struct {
	Pretty          bool   `json:"pretty"`
	Indent          string `json:"indent"`
	Prefix          string `json:"prefix"`
	EscapeHTML      bool   `json:"escape_html"`
	SortKeys        bool   `json:"sort_keys"`
	ValidateUTF8    bool   `json:"validate_utf8"`
	MaxDepth        int    `json:"max_depth"`
	DisallowUnknown bool   `json:"disallow_unknown"`

	// Number formatting
	PreserveNumbers bool `json:"preserve_numbers"`
	FloatPrecision  int  `json:"float_precision"`

	// Character escaping
	DisableEscaping bool `json:"disable_escaping"`
	EscapeUnicode   bool `json:"escape_unicode"`
	EscapeSlash     bool `json:"escape_slash"`
	EscapeNewlines  bool `json:"escape_newlines"`
	EscapeTabs      bool `json:"escape_tabs"`

	// Null handling
	IncludeNulls  bool            `json:"include_nulls"`
	CustomEscapes map[rune]string `json:"custom_escapes,omitempty"`
}

// Clone creates a deep copy of the EncodeConfig
func (c *EncodeConfig) Clone() *EncodeConfig {
	if c == nil {
		return DefaultEncodeConfig()
	}

	clone := *c

	if c.CustomEscapes != nil {
		clone.CustomEscapes = make(map[rune]string, len(c.CustomEscapes))
		for k, v := range c.CustomEscapes {
			clone.CustomEscapes[k] = v
		}
	}

	return &clone
}

// NewReadableConfig creates a configuration for human-readable JSON with minimal escaping
func NewReadableConfig() *EncodeConfig {
	return &EncodeConfig{
		Pretty:          false,
		Indent:          "  ",
		Prefix:          "",
		EscapeHTML:      false,
		SortKeys:        false,
		ValidateUTF8:    true,
		MaxDepth:        100,
		DisallowUnknown: false,
		DisableEscaping: true,
		EscapeUnicode:   false,
		EscapeSlash:     false,
		EscapeNewlines:  true,
		EscapeTabs:      true,
		IncludeNulls:    true, // Include null values by default
		CustomEscapes:   nil,
		PreserveNumbers: false, // Disable number preservation for encoding/json compatibility
	}
}

// NewWebSafeConfig creates a configuration for web-safe JSON
func NewWebSafeConfig() *EncodeConfig {
	return &EncodeConfig{
		Pretty:          false,
		Indent:          "  ",
		Prefix:          "",
		EscapeHTML:      true,
		SortKeys:        false,
		ValidateUTF8:    true,
		MaxDepth:        100,
		DisallowUnknown: false,
		DisableEscaping: false,
		EscapeUnicode:   false,
		EscapeSlash:     true,
		EscapeNewlines:  true,
		EscapeTabs:      true,
		IncludeNulls:    true, // Include null values by default
		CustomEscapes:   nil,
	}
}

// NewCleanConfig creates a configuration that omits null and empty values
func NewCleanConfig() *EncodeConfig {
	return &EncodeConfig{
		Pretty:          true,
		Indent:          "  ",
		Prefix:          "",
		EscapeHTML:      true,
		SortKeys:        true,
		ValidateUTF8:    true,
		MaxDepth:        100,
		DisallowUnknown: false,
		PreserveNumbers: false,
		FloatPrecision:  -1,
		DisableEscaping: false,
		EscapeUnicode:   false,
		EscapeSlash:     false,
		EscapeNewlines:  true,
		EscapeTabs:      true,
		IncludeNulls:    false, // Exclude null values for clean output
		CustomEscapes:   nil,
	}
}
