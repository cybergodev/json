package json

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

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
		num, err := parseInt(part)
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

// parseInt is a simple integer parser for validation
func parseInt(s string) (int, error) {
	var result int
	var negative bool
	i := 0

	if len(s) > 0 && s[0] == '-' {
		negative = true
		i = 1
	}

	for ; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return 0, fmt.Errorf("invalid integer")
		}
		result = result*10 + int(s[i]-'0')
	}

	if negative {
		result = -result
	}
	return result, nil
}
