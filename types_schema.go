package json

import (
	"fmt"
	"strconv"
)

// Schema represents a JSON schema for validation
type Schema struct {
	Type                 string             `json:"type,omitempty"`
	Properties           map[string]*Schema `json:"properties,omitempty"`
	Items                *Schema            `json:"items,omitempty"`
	Required             []string           `json:"required,omitempty"`
	MinLength            int                `json:"minLength,omitempty"`
	MaxLength            int                `json:"maxLength,omitempty"`
	Minimum              float64            `json:"minimum,omitempty"`
	Maximum              float64            `json:"maximum,omitempty"`
	Pattern              string             `json:"pattern,omitempty"`
	Format               string             `json:"format,omitempty"`
	AdditionalProperties bool               `json:"additionalProperties,omitempty"`
	MinItems             int                `json:"minItems,omitempty"`
	MaxItems             int                `json:"maxItems,omitempty"`
	UniqueItems          bool               `json:"uniqueItems,omitempty"`
	Enum                 []any              `json:"enum,omitempty"`
	Const                any                `json:"const,omitempty"`
	MultipleOf           float64            `json:"multipleOf,omitempty"`
	ExclusiveMinimum     bool               `json:"exclusiveMinimum,omitempty"`
	ExclusiveMaximum     bool               `json:"exclusiveMaximum,omitempty"`
	Title                string             `json:"title,omitempty"`
	Description          string             `json:"description,omitempty"`
	Default              any                `json:"default,omitempty"`
	Examples             []any              `json:"examples,omitempty"`

	// Internal flags
	hasMinLength bool
	hasMaxLength bool
	hasMinimum   bool
	hasMaximum   bool
	hasMinItems  bool
	hasMaxItems  bool
}

// ValidationError represents a schema validation error
type ValidationError struct {
	Path    string `json:"path"`
	Message string `json:"message"`
}

func (ve *ValidationError) Error() string {
	if ve.Path != "" {
		return fmt.Sprintf("validation error at path '%s': %s", ve.Path, ve.Message)
	}
	return fmt.Sprintf("validation error: %s", ve.Message)
}

// TypeSafeResult represents a type-safe operation result
type TypeSafeResult[T any] struct {
	Value  T
	Exists bool
	Error  error
}

// Ok returns true if the result is valid (no error and exists)
func (r TypeSafeResult[T]) Ok() bool {
	return r.Error == nil && r.Exists
}

// Unwrap returns the value or zero value if there's an error
// For panic behavior, use UnwrapOrPanic instead
func (r TypeSafeResult[T]) Unwrap() T {
	if r.Error != nil {
		var zero T
		return zero
	}
	return r.Value
}

// UnwrapOrPanic returns the value or panics if there's an error
// Use this only when you're certain the operation succeeded
func (r TypeSafeResult[T]) UnwrapOrPanic() T {
	if r.Error != nil {
		panic(fmt.Sprintf("unwrap called on result with error: %v", r.Error))
	}
	return r.Value
}

// UnwrapOr returns the value or the provided default if there's an error or value doesn't exist
func (r TypeSafeResult[T]) UnwrapOr(defaultValue T) T {
	if r.Error != nil || !r.Exists {
		return defaultValue
	}
	return r.Value
}

// TypeSafeAccessResult represents the result of a type-safe access operation
type TypeSafeAccessResult struct {
	Value  any
	Exists bool
	Type   string
}

// AsString safely converts the result to string
func (r TypeSafeAccessResult) AsString() (string, error) {
	if !r.Exists {
		return "", ErrPathNotFound
	}
	if str, ok := r.Value.(string); ok {
		return str, nil
	}
	return fmt.Sprintf("%v", r.Value), nil
}

// AsInt safely converts the result to int
func (r TypeSafeAccessResult) AsInt() (int, error) {
	if !r.Exists {
		return 0, ErrPathNotFound
	}

	switch v := r.Value.(type) {
	case int:
		return v, nil
	case int64:
		return int(v), nil
	case float64:
		return int(v), nil
	case string:
		return strconv.Atoi(v)
	default:
		return 0, fmt.Errorf("cannot convert %T to int", v)
	}
}

// AsBool safely converts the result to bool
func (r TypeSafeAccessResult) AsBool() (bool, error) {
	if !r.Exists {
		return false, ErrPathNotFound
	}

	switch v := r.Value.(type) {
	case bool:
		return v, nil
	case string:
		return strconv.ParseBool(v)
	default:
		return false, fmt.Errorf("cannot convert %T to bool", v)
	}
}

// DefaultSchema returns a default schema configuration
func DefaultSchema() *Schema {
	return &Schema{
		Type:                 "",
		Properties:           make(map[string]*Schema),
		Items:                nil,
		Required:             []string{},
		MinLength:            0,
		MaxLength:            0,
		Minimum:              0,
		Maximum:              0,
		Pattern:              "",
		Format:               "",
		AdditionalProperties: true,
		MinItems:             0,
		MaxItems:             0,
	}
}

// SetMinLength sets the minimum length constraint
func (s *Schema) SetMinLength(minLength int) *Schema {
	s.MinLength = minLength
	s.hasMinLength = true
	return s
}

// SetMaxLength sets the maximum length constraint
func (s *Schema) SetMaxLength(maxLength int) *Schema {
	s.MaxLength = maxLength
	s.hasMaxLength = true
	return s
}

// SetMinimum sets the minimum value constraint
func (s *Schema) SetMinimum(minimum float64) *Schema {
	s.Minimum = minimum
	s.hasMinimum = true
	return s
}

// SetMaximum sets the maximum value constraint
func (s *Schema) SetMaximum(maximum float64) *Schema {
	s.Maximum = maximum
	s.hasMaximum = true
	return s
}

// HasMinLength returns true if MinLength constraint is explicitly set
func (s *Schema) HasMinLength() bool {
	return s.hasMinLength
}

// HasMaxLength returns true if MaxLength constraint is explicitly set
func (s *Schema) HasMaxLength() bool {
	return s.hasMaxLength
}

// HasMinimum returns true if Minimum constraint is explicitly set
func (s *Schema) HasMinimum() bool {
	return s.hasMinimum
}

// HasMaximum returns true if Maximum constraint is explicitly set
func (s *Schema) HasMaximum() bool {
	return s.hasMaximum
}

// SetMinItems sets the minimum items constraint for arrays
func (s *Schema) SetMinItems(minItems int) *Schema {
	s.MinItems = minItems
	s.hasMinItems = true
	return s
}

// SetMaxItems sets the maximum items constraint for arrays
func (s *Schema) SetMaxItems(maxItems int) *Schema {
	s.MaxItems = maxItems
	s.hasMaxItems = true
	return s
}

// HasMinItems returns true if MinItems constraint is explicitly set
func (s *Schema) HasMinItems() bool {
	return s.hasMinItems
}

// HasMaxItems returns true if MaxItems constraint is explicitly set
func (s *Schema) HasMaxItems() bool {
	return s.hasMaxItems
}

// SetExclusiveMinimum sets the exclusive minimum flag
func (s *Schema) SetExclusiveMinimum(exclusive bool) *Schema {
	s.ExclusiveMinimum = exclusive
	return s
}

// SetExclusiveMaximum sets the exclusive maximum flag
func (s *Schema) SetExclusiveMaximum(exclusive bool) *Schema {
	s.ExclusiveMaximum = exclusive
	return s
}
