package json

import (
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

// Lazy-initialized regex patterns for schema validation
// PERFORMANCE: Using sync.OnceValue to defer compilation until first use
// This avoids init-time overhead when schema validation isn't needed
var (
	// emailLocalRegex validates local part of email addresses
	emailLocalRegex = sync.OnceValue(func() *regexp.Regexp {
		return regexp.MustCompile(`^[a-zA-Z0-9._%+-]+$`)
	})
	// emailDomainRegex validates domain part of email addresses
	emailDomainRegex = sync.OnceValue(func() *regexp.Regexp {
		return regexp.MustCompile(`^[a-zA-Z0-9.-]+$`)
	})
	// uuidRegex validates UUID format (v4 pattern)
	uuidRegex = sync.OnceValue(func() *regexp.Regexp {
		return regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)
	})
	// ipv6Regex validates IPv6 address format
	ipv6Regex = sync.OnceValue(func() *regexp.Regexp {
		return regexp.MustCompile(`^([0-9a-fA-F]{0,4}:){2,7}[0-9a-fA-F]{0,4}$`)
	})
)

// ValidateSchema validates JSON data against a schema
func (p *Processor) ValidateSchema(jsonStr string, schema *Schema, cfg ...Config) ([]ValidationError, error) {
	if err := p.checkClosed(); err != nil {
		return nil, err
	}

	options, err := p.prepareOptions(cfg...)
	if err != nil {
		return nil, err
	}
	defer releaseConfig(options)

	if err := p.validateInput(jsonStr); err != nil {
		return nil, err
	}

	if schema == nil {
		return nil, &JsonsError{
			Op:      "validate_schema",
			Message: "schema cannot be nil",
			Err:     errOperationFailed,
		}
	}

	// Parse JSON
	var data any
	err = p.Parse(jsonStr, &data, *options)
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
		case int, int8, int16, int32, int64,
			uint, uint8, uint16, uint32, uint64,
			float32, float64:
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
	if schema.hasMinItems && arrLen < schema.MinItems {
		*errors = append(*errors, ValidationError{
			Path:    path,
			Message: fmt.Sprintf("array length %d is less than minimum %d", arrLen, schema.MinItems),
		})
	}

	if schema.hasMaxItems && arrLen > schema.MaxItems {
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
	strLen := utf8.RuneCountInString(str)
	if schema.hasMinLength && strLen < schema.MinLength {
		*errors = append(*errors, ValidationError{
			Path:    path,
			Message: fmt.Sprintf("string length %d is less than minimum %d", strLen, schema.MinLength),
		})
	}

	if schema.hasMaxLength && strLen > schema.MaxLength {
		*errors = append(*errors, ValidationError{
			Path:    path,
			Message: fmt.Sprintf("string length %d exceeds maximum %d", strLen, schema.MaxLength),
		})
	}

	// Pattern validation (regular expression)
	// Lazily compile and cache the regex to prevent recompilation on every call
	if schema.Pattern != "" {
		if !schema.patternCompiled {
			re, err := regexp.Compile(schema.Pattern)
			if err != nil {
				*errors = append(*errors, ValidationError{
					Path:    path,
					Message: fmt.Sprintf("invalid pattern '%s': %v", schema.Pattern, err),
				})
				return
			}
			schema.compiledPattern = re
			schema.patternCompiled = true
		}
		if schema.compiledPattern != nil {
			if !schema.compiledPattern.MatchString(str) {
				*errors = append(*errors, ValidationError{
					Path:    path,
					Message: fmt.Sprintf("string '%s' does not match pattern '%s'", str, schema.Pattern),
				})
			}
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
	if schema.hasMinimum {
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

	if schema.hasMaximum {
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
		// Use tolerance-based comparison to handle IEEE 754 floating-point imprecision
		quotient := num / schema.MultipleOf
		roundedQuotient := float64(int(quotient + 0.5))
		remainder := num - schema.MultipleOf*roundedQuotient
		const epsilon = 1e-9
		if remainder < -epsilon || remainder > epsilon {
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
	if !emailLocalRegex().MatchString(localPart) || !emailDomainRegex().MatchString(domainPart) {
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
	if !uuidRegex().MatchString(uuid) {
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

	// Use lazy-initialized regex for validation
	if !ipv6Regex().MatchString(ip) {
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
