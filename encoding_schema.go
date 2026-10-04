package json

import (
	"encoding/json"
	"fmt"
	"maps"
	"math"
	"net"
	"reflect"
	"regexp"
	"slices"
	"strconv"
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
)

// ValidateSchema validates JSON data against a schema.
//
// Schema violations are reported as the returned []ValidationError slice;
// the error return value is non-nil only for parse or setup failures.
//
// Errors:
//   - ErrProcessorClosed: processor has been closed
//   - ErrInvalidJSON: jsonStr is not valid JSON
//   - ErrSizeLimit: jsonStr exceeds MaxJSONSize
//   - a JsonsError (errOperationFailed) when schema is nil
func (p *Processor) ValidateSchema(jsonStr string, schema *Schema, cfg ...Config) ([]ValidationError, error) {
	if err := p.checkClosed(); err != nil {
		return nil, err
	}

	options, err := p.prepareOptions(cfg...)
	if err != nil {
		return nil, err
	}
	defer releaseConfig(options)

	if err := p.validateInputForOptions(jsonStr, options); err != nil {
		return nil, err
	}

	if schema == nil {
		return nil, &JsonsError{
			Op:      "validate_schema",
			Message: "schema cannot be nil",
			Err:     errOperationFailed,
		}
	}

	// Parse JSON. parseJSON (not p.Parse): the input was validated above, so
	// the sentinel-dereferencing p.Parse call would re-validate through a
	// transient securityValidator on every schema check (P-001).
	data, err := p.parseJSON(jsonStr, "validate_schema", "", options)
	if err != nil {
		return nil, err
	}

	// Valid against schema
	var errors []ValidationError
	p.validateValue(data, schema, "", &errors, 0)

	return errors, nil
}

// validateValue validates a value against a schema with improved performance.
// depth bounds the schema recursion: Schema is a recursive type, so a
// self-referential schema (s.Items = s) would otherwise recurse until the
// goroutine stack is exhausted — a fatal, unrecoverable crash.
func (p *Processor) validateValue(value any, schema *Schema, path string, errors *[]ValidationError, depth int) {
	if schema == nil {
		return
	}

	if depth > DefaultMaxNestingDepth {
		*errors = append(*errors, ValidationError{
			Path:    path,
			Message: fmt.Sprintf("schema nesting exceeds maximum depth %d", DefaultMaxNestingDepth),
		})
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
			p.validateObject(obj, schema, path, errors, depth)
		}
	case "array":
		if arr, ok := value.([]any); ok {
			p.validateArray(arr, schema, path, errors, depth)
		}
	case "string":
		if str, ok := value.(string); ok {
			p.validateString(str, schema, path, errors)
		}
	case "number", "integer":
		// "integer" values passed validateType above (integral only); their
		// numeric range constraints share validateNumber with "number".
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
			float32, float64,
			Number, json.Number:
			// Number/json.Number appear when the document was parsed with
			// PreserveNumbers; without them EVERY number failed type checks
			// ("expected type number, got json.Number") under that config.
			return true
		}
		return false
	case "integer":
		// JSON Schema semantics: any mathematically integral value qualifies
		// (1, 1.0, and 1e2 are all valid integers), so floats are accepted when
		// they carry no fractional part. Number literals are checked by value
		// via Float64, mirroring validateNumber's PreserveNumbers handling.
		// Previously this case was missing and "integer" schemas rejected
		// every input, including valid integers (D-002).
		switch v := value.(type) {
		case int, int8, int16, int32, int64,
			uint, uint8, uint16, uint32, uint64:
			return true
		case float32:
			f := float64(v)
			return !math.IsNaN(f) && !math.IsInf(f, 0) && f == math.Trunc(f)
		case float64:
			return !math.IsNaN(v) && !math.IsInf(v, 0) && v == math.Trunc(v)
		case Number:
			f, err := v.Float64()
			return err == nil && f == math.Trunc(f)
		case json.Number:
			f, err := v.Float64()
			return err == nil && f == math.Trunc(f)
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
func (p *Processor) validateObject(obj map[string]any, schema *Schema, path string, errors *[]ValidationError, depth int) {
	// Required properties validation
	for _, required := range schema.Required {
		if _, exists := obj[required]; !exists {
			*errors = append(*errors, ValidationError{
				Path:    p.joinPath(path, required),
				Message: fmt.Sprintf("required property '%s' is missing", required),
			})
		}
	}

	// Valid properties — in sorted key order so the reported error list is
	// deterministic (map iteration order is randomized per call, which
	// shuffled the order of "additional property" and nested errors).
	for _, key := range slices.Sorted(maps.Keys(obj)) {
		val := obj[key]
		if propSchema, exists := schema.Properties[key]; exists {
			p.validateValue(val, propSchema, p.joinPath(path, key), errors, depth+1)
		} else if !schema.AdditionalProperties {
			*errors = append(*errors, ValidationError{
				Path:    p.joinPath(path, key),
				Message: fmt.Sprintf("additional property '%s' is not allowed", key),
			})
		}
	}
}

// validateArray validates an array against a schema with type safety
func (p *Processor) validateArray(arr []any, schema *Schema, path string, errors *[]ValidationError, depth int) {
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
			// Include the dynamic type in the key: %v alone collides across
			// JSON types ([1, "1"] both render "1"), producing spurious
			// "duplicate item" errors for distinct values.
			itemStr := fmt.Sprintf("%T:%v", item, item)
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
			p.validateValue(item, schema.Items, itemPath, errors, depth+1)
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
	// Lazily compile and cache the regex via sync.Once so a shared *Schema is safe
	// for concurrent validation. (*regexp.Regexp).MatchString is itself goroutine-safe,
	// so only the one-time compile needs guarding. A compile error is cached and
	// re-reported on every validation rather than silently passing.
	if schema.Pattern != "" {
		schema.patternOnce.Do(func() {
			schema.compiledPattern, schema.patternErr = regexp.Compile(schema.Pattern)
		})
		if schema.patternErr != nil {
			*errors = append(*errors, ValidationError{
				Path:    path,
				Message: fmt.Sprintf("invalid pattern '%s': %v", schema.Pattern, schema.patternErr),
			})
			return
		}
		if !schema.compiledPattern.MatchString(str) {
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
	// Cover every numeric kind validateType accepts for "number"; skipping a
	// kind here would silently skip the range constraints for that type.
	switch v := value.(type) {
	case int:
		num = float64(v)
	case int8:
		num = float64(v)
	case int16:
		num = float64(v)
	case int32:
		num = float64(v)
	case int64:
		num = float64(v)
	case uint:
		num = float64(v)
	case uint8:
		num = float64(v)
	case uint16:
		num = float64(v)
	case uint32:
		num = float64(v)
	case uint64:
		num = float64(v)
	case float32:
		num = float64(v)
	case float64:
		num = v
	case Number:
		// PreserveNumbers parses yield the library's Number; without these
		// cases validateNumber hit `default: return` and silently skipped
		// minimum/maximum/multipleOf for every number in the document.
		f, err := v.Float64()
		if err != nil {
			return
		}
		num = f
	case json.Number:
		f, err := v.Float64()
		if err != nil {
			return
		}
		num = f
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
		// Use tolerance-based comparison to handle IEEE 754 floating-point imprecision.
		// math.Round (not int(q+0.5)) avoids the implementation-defined int(±Inf)
		// conversion — e.g. num=1e300, MultipleOf=1e-300 produced garbage results —
		// and rounds negative quotients symmetrically.
		quotient := num / schema.MultipleOf
		roundedQuotient := math.Round(quotient)
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

// validateIPv6Format validates IPv6 format.
// Delegates to the standard library so every legal form — including zero-compression
// ("::"), e.g. "fe80::1" or "2001:db8::1" — is accepted, while IPv4 and non-IP
// strings are rejected.
func (p *Processor) validateIPv6Format(ip, path string, errors *[]ValidationError) error {
	parsed := net.ParseIP(ip)
	if parsed == nil || parsed.To4() != nil || !strings.Contains(ip, ":") {
		*errors = append(*errors, ValidationError{
			Path:    path,
			Message: fmt.Sprintf("'%s' is not a valid IPv6 format", ip),
		})
	}
	return nil
}

// valuesEqual compares two values for equality.
//
// Const/Enum constraints are `any` and routinely hold objects or arrays, so
// a bare `a == b` is unsafe: comparing two interface values with identical
// non-comparable dynamic types (e.g. two map[string]any) panics at runtime.
// Primitives are compared with == only when both sides are comparably typed;
// everything else falls through to a structural comparison.
func (p *Processor) valuesEqual(a, b any) bool {
	// Handle nil cases
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}

	// Direct comparison for basic types
	if isComparableValue(a) && isComparableValue(b) && a == b {
		return true
	}

	// Structural comparison for container constraints (objects/arrays)
	if reflect.DeepEqual(a, b) {
		return true
	}

	// Handle numeric type conversions. The nested switch below only matched
	// int/float64 as the LEFT operand, so an enum constant held as int64/uint/
	// float32 never compared equal to the parsed float64 document value.
	// Normalize both sides to float64 first.
	fa, aok := numericAsFloat(a)
	fb, bok := numericAsFloat(b)
	if aok && bok {
		return fa == fb
	}

	return false
}

// numericAsFloat converts any numeric value (including the Number/json.Number
// forms produced by PreserveNumbers parsing) to float64.
func numericAsFloat(v any) (float64, bool) {
	switch n := v.(type) {
	case int:
		return float64(n), true
	case int8:
		return float64(n), true
	case int16:
		return float64(n), true
	case int32:
		return float64(n), true
	case int64:
		return float64(n), true
	case uint:
		return float64(n), true
	case uint8:
		return float64(n), true
	case uint16:
		return float64(n), true
	case uint32:
		return float64(n), true
	case uint64:
		return float64(n), true
	case float32:
		return float64(n), true
	case float64:
		return n, true
	case Number:
		f, err := n.Float64()
		return f, err == nil
	case json.Number:
		f, err := n.Float64()
		return f, err == nil
	}
	return 0, false
}

// isComparableValue reports whether v's dynamic type supports == without a
// runtime panic (maps, slices, and funcs are not comparable).
func isComparableValue(v any) bool {
	switch v.(type) {
	case map[string]any, []any:
		return false
	}
	return reflect.TypeOf(v).Comparable()
}

// joinPath joins path segments
func (p *Processor) joinPath(parent, child string) string {
	if parent == "" {
		return child
	}
	return parent + "." + child
}

// parseInt is a simple integer parser for validation.
// Rejects empty strings, lone signs, non-digit characters, and values that
// overflow int. (The previous implementation returned 0 for "" and "-",
// letting "1..2.3" pass as a valid IPv4 address, and silently wrapped
// out-of-range values.)
func parseInt(s string) (int, error) {
	if s == "" {
		return 0, fmt.Errorf("invalid integer: empty string")
	}
	body := s
	if body[0] == '-' {
		body = body[1:]
	}
	if body == "" {
		return 0, fmt.Errorf("invalid integer: %q", s)
	}
	for i := 0; i < len(body); i++ {
		if body[i] < '0' || body[i] > '9' {
			return 0, fmt.Errorf("invalid integer: %q", s)
		}
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0, fmt.Errorf("integer out of range: %q", s)
	}
	return n, nil
}
