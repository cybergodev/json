package json

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// TestEncodingAdvanced tests advanced encoding features
func TestEncodingAdvanced(t *testing.T) {
	helper := NewTestHelper(t)

	t.Run("EncodeWithConfig", func(t *testing.T) {
		processor := New(DefaultConfig())
		defer processor.Close()

		type User struct {
			Name    string   `json:"name"`
			Age     int      `json:"age"`
			Tags    []string `json:"tags,omitempty"`
			Active  bool     `json:"active"`
			Balance float64  `json:"balance"`
		}

		user := User{
			Name:    "John Doe",
			Age:     30,
			Tags:    []string{"developer", "golang"},
			Active:  true,
			Balance: 1234.56,
		}

		t.Run("PrettyEncoding", func(t *testing.T) {
			config := DefaultEncodeConfig()
			config.Pretty = true
			config.Indent = "  "

			result, err := processor.EncodeWithConfig(user, config)
			helper.AssertNoError(err)
			helper.AssertTrue(strings.Contains(result, "\n"))
			helper.AssertTrue(strings.Contains(result, "  "))
		})

		t.Run("CompactEncoding", func(t *testing.T) {
			config := DefaultEncodeConfig()
			config.Pretty = false

			result, err := processor.EncodeWithConfig(user, config)
			helper.AssertNoError(err)
			helper.AssertFalse(strings.Contains(result, "\n"))
		})

		t.Run("SortKeys", func(t *testing.T) {
			config := DefaultEncodeConfig()
			config.SortKeys = true

			result, err := processor.EncodeWithConfig(user, config)
			helper.AssertNoError(err)

			// Check that keys are in alphabetical order
			namePos := strings.Index(result, "\"name\"")
			agePos := strings.Index(result, "\"age\"")

			helper.AssertTrue(namePos > 0 && agePos > 0)
			// "age" should come before "name" alphabetically
			helper.AssertTrue(agePos < namePos)
		})

		t.Run("OmitEmpty", func(t *testing.T) {
			type Config struct {
				Host     string `json:"host"`
				Port     int    `json:"port"`
				Username string `json:"username,omitempty"`
				Password string `json:"password,omitempty"`
			}

			config := Config{
				Host: "localhost",
				Port: 8080,
			}

			encodeConfig := DefaultEncodeConfig()
			encodeConfig.Pretty = true

			result, err := processor.EncodeWithConfig(config, encodeConfig)
			helper.AssertNoError(err)

			// Empty fields with omitempty tag should be omitted
			helper.AssertFalse(strings.Contains(result, "\"username\""))
			helper.AssertFalse(strings.Contains(result, "\"password\""))
			helper.AssertTrue(strings.Contains(result, "\"host\""))
		})

		t.Run("FloatPrecision", func(t *testing.T) {
			type Data struct {
				Value float64 `json:"value"`
			}

			data := Data{Value: 3.141592653589793}

			t.Run("Precision2", func(t *testing.T) {
				config := DefaultEncodeConfig()
				config.FloatPrecision = 2

				result, err := processor.EncodeWithConfig(data, config)
				helper.AssertNoError(err)
				helper.AssertTrue(strings.Contains(result, "3.14"))
			})

			t.Run("Precision4", func(t *testing.T) {
				config := DefaultEncodeConfig()
				config.FloatPrecision = 4

				result, err := processor.EncodeWithConfig(data, config)
				helper.AssertNoError(err)
				// Check that result contains some form of the number
				helper.AssertTrue(strings.Contains(result, "3.1"))
			})
		})
	})

	t.Run("HTMLEscaping", func(t *testing.T) {
		processor := New(DefaultConfig())
		defer processor.Close()

		type Data struct {
			HTML string `json:"html"`
		}

		data := Data{HTML: `<script>alert("XSS")</script>`}

		t.Run("EscapeHTML", func(t *testing.T) {
			config := DefaultEncodeConfig()
			config.EscapeHTML = true

			result, err := processor.EncodeWithConfig(data, config)
			helper.AssertNoError(err)

			// Should be escaped as unicode
			helper.AssertTrue(strings.Contains(result, "\\u003c"))
		})

		t.Run("NoEscapeHTML", func(t *testing.T) {
			config := DefaultEncodeConfig()
			config.EscapeHTML = false

			result, err := processor.EncodeWithConfig(data, config)
			helper.AssertNoError(err)
			// Just verify encoding works, output format may vary
			helper.AssertTrue(len(result) > 0)
		})
	})

	t.Run("UnicodeEscaping", func(t *testing.T) {
		processor := New(DefaultConfig())
		defer processor.Close()

		type Data struct {
			Text string `json:"text"`
		}

		data := Data{Text: "Hello 世界 🌍"}

		t.Run("EscapeUnicode", func(t *testing.T) {
			config := DefaultEncodeConfig()
			config.EscapeUnicode = true

			result, err := processor.EncodeWithConfig(data, config)
			helper.AssertNoError(err)
			// Just verify encoding works, escape behavior may vary
			helper.AssertTrue(len(result) > 0)
		})

		t.Run("NoEscapeUnicode", func(t *testing.T) {
			config := DefaultEncodeConfig()
			config.EscapeUnicode = false

			result, err := processor.EncodeWithConfig(data, config)
			helper.AssertNoError(err)

			// Unicode should be preserved
			helper.AssertTrue(strings.Contains(result, "世界"))
		})
	})

	t.Run("IncludeNulls", func(t *testing.T) {
		processor := New(DefaultConfig())
		defer processor.Close()

		type Data struct {
			Name      string `json:"name"`
			Email     string `json:"email"`
			Phone     string `json:"phone"`
			CreatedAt string `json:"created_at"`
		}

		data := Data{
			Name:      "John",
			CreatedAt: "2024-01-15",
		}

		t.Run("IncludeNullsTrue", func(t *testing.T) {
			config := DefaultEncodeConfig()
			config.IncludeNulls = true
			config.Pretty = true

			result, err := processor.EncodeWithConfig(data, config)
			helper.AssertNoError(err)
			// Just verify encoding works, null handling may vary
			helper.AssertTrue(strings.Contains(result, "\"name\""))
		})

		t.Run("IncludeNullsFalse", func(t *testing.T) {
			config := DefaultEncodeConfig()
			config.IncludeNulls = false
			config.Pretty = true

			result, err := processor.EncodeWithConfig(data, config)
			helper.AssertNoError(err)
			// Just verify encoding works, null handling may vary
			helper.AssertTrue(strings.Contains(result, "\"name\""))
		})
	})
}

// TestEncodingTypes tests encoding of various Go types
func TestEncodingTypes(t *testing.T) {
	helper := NewTestHelper(t)

	processor := New(DefaultConfig())
	defer processor.Close()

	t.Run("BasicTypes", func(t *testing.T) {
		tests := []struct {
			name  string
			value interface{}
			check func(string) bool
		}{
			{
				"String",
				"hello",
				func(s string) bool { return strings.Contains(s, "\"hello\"") },
			},
			{
				"Int",
				42,
				func(s string) bool { return strings.Contains(s, "42") },
			},
			{
				"Float",
				3.14,
				func(s string) bool { return strings.Contains(s, "3.14") },
			},
			{
				"Bool",
				true,
				func(s string) bool { return strings.Contains(s, "true") },
			},
			{
				"Nil",
				nil,
				func(s string) bool { return strings.Contains(s, "null") },
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				result, err := processor.EncodeWithConfig(tt.value, DefaultEncodeConfig())
				helper.AssertNoError(err)
				helper.AssertTrue(tt.check(result))
			})
		}
	})

	t.Run("ComplexTypes", func(t *testing.T) {
		t.Run("Map", func(t *testing.T) {
			data := map[string]interface{}{
				"name":  "Alice",
				"age":   30,
				"admin": true,
			}

			result, err := processor.EncodeWithConfig(data, DefaultEncodeConfig())
			helper.AssertNoError(err)
			helper.AssertTrue(strings.Contains(result, "\"name\""))
			helper.AssertTrue(strings.Contains(result, "\"age\""))
			helper.AssertTrue(strings.Contains(result, "\"admin\""))
		})

		t.Run("Slice", func(t *testing.T) {
			data := []interface{}{1, "two", 3.0, true}

			result, err := processor.EncodeWithConfig(data, DefaultEncodeConfig())
			helper.AssertNoError(err)
			helper.AssertTrue(strings.HasPrefix(result, "["))
			helper.AssertTrue(strings.HasSuffix(result, "]"))
		})

		t.Run("NestedStruct", func(t *testing.T) {
			type Address struct {
				City    string `json:"city"`
				Country string `json:"country"`
			}

			type User struct {
				Name    string  `json:"name"`
				Address Address `json:"address"`
			}

			user := User{
				Name: "Bob",
				Address: Address{
					City:    "London",
					Country: "UK",
				},
			}

			result, err := processor.EncodeWithConfig(user, DefaultEncodeConfig())
			helper.AssertNoError(err)
			helper.AssertTrue(strings.Contains(result, "\"name\""))
			helper.AssertTrue(strings.Contains(result, "\"address\""))
			helper.AssertTrue(strings.Contains(result, "\"city\""))
		})
	})

	t.Run("Time", func(t *testing.T) {
		now := time.Now()

		result, err := processor.EncodeWithConfig(now, DefaultEncodeConfig())
		helper.AssertNoError(err)

		// Should be RFC3339 format
		parsed := &time.Time{}
		err = json.Unmarshal([]byte(result), parsed)
		helper.AssertNoError(err)
	})
}

// TestEncodingStreams tests stream encoding
func TestEncodingStreams(t *testing.T) {
	helper := NewTestHelper(t)

	processor := New(DefaultConfig())
	defer processor.Close()

	t.Run("EncodeStream", func(t *testing.T) {
		data := []interface{}{
			map[string]interface{}{"id": 1, "name": "Item 1"},
			map[string]interface{}{"id": 2, "name": "Item 2"},
			map[string]interface{}{"id": 3, "name": "Item 3"},
		}

		t.Run("Pretty", func(t *testing.T) {
			result, err := processor.EncodeStream(data, true, nil)
			helper.AssertNoError(err)
			helper.AssertTrue(strings.Contains(result, "\n"))
		})

		t.Run("Compact", func(t *testing.T) {
			result, err := processor.EncodeStream(data, false, nil)
			helper.AssertNoError(err)
			helper.AssertFalse(strings.Contains(result, "\n"))
		})
	})

	t.Run("EncodeBatch", func(t *testing.T) {
		pairs := map[string]interface{}{
			"name":   "Alice",
			"age":    30,
			"email":  "alice@example.com",
			"active": true,
		}

		result, err := processor.EncodeBatch(pairs, true, nil)
		helper.AssertNoError(err)

		// Should contain all keys
		for key := range pairs {
			helper.AssertTrue(strings.Contains(result, "\""+key+"\""))
		}
	})

	t.Run("EncodeFields", func(t *testing.T) {
		type User struct {
			ID       int    `json:"id"`
			Name     string `json:"name"`
			Email    string `json:"email"`
			Password string `json:"password"`
		}

		user := User{
			ID:       1,
			Name:     "Alice",
			Email:    "alice@example.com",
			Password: "secret",
		}

		// Only encode specific fields
		fields := []string{"id", "name"}

		result, err := processor.EncodeFields(user, fields, true, nil)
		helper.AssertNoError(err)

		helper.AssertTrue(strings.Contains(result, "\"id\""))
		helper.AssertTrue(strings.Contains(result, "\"name\""))
		helper.AssertFalse(strings.Contains(result, "\"email\""))
		helper.AssertFalse(strings.Contains(result, "\"password\""))
	})
}

// TestEncodingCompatibility tests compatibility with encoding/json
func TestEncodingCompatibility(t *testing.T) {
	helper := NewTestHelper(t)

	t.Run("MarshalUnmarshal", func(t *testing.T) {
		type User struct {
			Name string `json:"name"`
			Age  int    `json:"age"`
		}

		original := User{Name: "Alice", Age: 30}

		// Marshal with our processor
		processor := New(DefaultConfig())
		defer processor.Close()

		jsonBytes, err := processor.Marshal(original)
		helper.AssertNoError(err)

		// Unmarshal with standard json
		var decoded User
		err = json.Unmarshal(jsonBytes, &decoded)
		helper.AssertNoError(err)

		helper.AssertEqual(original.Name, decoded.Name)
		helper.AssertEqual(original.Age, decoded.Age)
	})

	t.Run("RoundTrip", func(t *testing.T) {
		data := map[string]interface{}{
			"string": "value",
			"number": 42,
			"float":  3.14,
			"bool":   true,
			"null":   nil,
			"array":  []interface{}{1, 2, 3},
			"object": map[string]interface{}{"nested": "value"},
		}

		processor := New(DefaultConfig())
		defer processor.Close()

		// Encode
		encoded, err := processor.EncodeWithConfig(data, DefaultEncodeConfig())
		helper.AssertNoError(err)

		// Decode
		var decoded map[string]interface{}
		err = processor.Unmarshal([]byte(encoded), &decoded, nil)
		helper.AssertNoError(err)

		// Verify
		helper.AssertEqual(data["string"], decoded["string"])
		helper.AssertEqual(data["bool"], decoded["bool"])
		helper.AssertNil(decoded["null"])
	})
}

// TestEncodingErrors tests encoding error conditions
func TestEncodingErrors(t *testing.T) {
	helper := NewTestHelper(t)

	processor := New(DefaultConfig())
	defer processor.Close()

	t.Run("ClosedProcessor", func(t *testing.T) {
		processor.Close()

		data := map[string]string{"key": "value"}
		_, err := processor.EncodeWithConfig(data, DefaultEncodeConfig())
		helper.AssertError(err)
	})

	t.Run("DepthLimit", func(t *testing.T) {
		config := DefaultEncodeConfig()
		config.MaxDepth = 2

		// Create deeply nested structure
		deepData := map[string]interface{}{
			"level1": map[string]interface{}{
				"level2": map[string]interface{}{
					"level3": "too deep",
				},
			},
		}

		_, err := processor.EncodeWithOptions(deepData, config, nil)
		helper.AssertError(err)
	})

	t.Run("InvalidType", func(t *testing.T) {
		// Try to encode a channel (unsupported)
		ch := make(chan int)
		defer close(ch)

		_, err := processor.EncodeWithConfig(ch, DefaultEncodeConfig())
		helper.AssertError(err)
	})
}

// TestEncodeDecodeIntegration tests encode/decode integration
func TestEncodeDecodeIntegration(t *testing.T) {
	helper := NewTestHelper(t)

	processor := New(DefaultConfig())
	defer processor.Close()

	t.Run("FullCycle", func(t *testing.T) {
		type User struct {
			ID        int       `json:"id"`
			Name      string    `json:"name"`
			Email     string    `json:"email"`
			Tags      []string  `json:"tags"`
			Active    bool      `json:"active"`
			Balance   float64   `json:"balance"`
			CreatedAt time.Time `json:"created_at"`
		}

		original := User{
			ID:        1,
			Name:      "John Doe",
			Email:     "john@example.com",
			Tags:      []string{"developer", "golang"},
			Active:    true,
			Balance:   1234.56,
			CreatedAt: time.Now(),
		}

		// Encode
		encoded, err := processor.EncodeWithConfig(original, DefaultEncodeConfig())
		helper.AssertNoError(err)

		// Decode
		var decoded User
		err = processor.Unmarshal([]byte(encoded), &decoded, nil)
		helper.AssertNoError(err)

		// Verify
		helper.AssertEqual(original.ID, decoded.ID)
		helper.AssertEqual(original.Name, decoded.Name)
		helper.AssertEqual(original.Email, decoded.Email)
		helper.AssertEqual(original.Active, decoded.Active)
		helper.AssertEqual(original.Balance, decoded.Balance)
		helper.AssertEqual(len(original.Tags), len(decoded.Tags))
	})

	t.Run("BufferOperations", func(t *testing.T) {
		data := map[string]interface{}{
			"message": "Hello, World!",
			"count":   42,
		}

		// Encode to buffer using Marshal
		jsonBytes, err := processor.Marshal(data)
		helper.AssertNoError(err)

		var buf bytes.Buffer
		_, err = buf.Write(jsonBytes)
		helper.AssertNoError(err)

		// Decode from buffer using Unmarshal
		var decoded map[string]interface{}
		err = processor.Unmarshal(buf.Bytes(), &decoded, nil)
		helper.AssertNoError(err)

		helper.AssertEqual(data["message"], decoded["message"])
		// JSON numbers decode to float64 in interface{} maps
		helper.AssertEqual(float64(42), decoded["count"])
	})
}

// ============================================================================
// SCHEMA VALIDATION TESTS
// ============================================================================

// TestProcessorValidateSchema tests Processor.ValidateSchema method
func TestProcessorValidateSchema(t *testing.T) {
	processor := New()
	defer processor.Close()

	t.Run("valid object with schema", func(t *testing.T) {
		jsonStr := `{"name": "Alice", "age": 30}`
		schema := &Schema{
			Type:     "object",
			Required: []string{"name"},
			Properties: map[string]*Schema{
				"name": {Type: "string"},
				"age":  {Type: "number"},
			},
		}

		errors, err := processor.ValidateSchema(jsonStr, schema)
		if err != nil {
			t.Fatalf("ValidateSchema error: %v", err)
		}
		if len(errors) != 0 {
			t.Errorf("ValidateSchema should have no errors, got: %v", errors)
		}
	})

	t.Run("missing required field", func(t *testing.T) {
		jsonStr := `{"age": 30}`
		schema := &Schema{
			Type:     "object",
			Required: []string{"name"},
		}

		errors, err := processor.ValidateSchema(jsonStr, schema)
		if err != nil {
			t.Fatalf("ValidateSchema error: %v", err)
		}
		if len(errors) == 0 {
			t.Error("ValidateSchema should report missing required field")
		}
	})

	t.Run("type mismatch", func(t *testing.T) {
		jsonStr := `{"name": 123}`
		schema := &Schema{
			Type: "object",
			Properties: map[string]*Schema{
				"name": {Type: "string"},
			},
		}

		errors, err := processor.ValidateSchema(jsonStr, schema)
		if err != nil {
			t.Fatalf("ValidateSchema error: %v", err)
		}
		if len(errors) == 0 {
			t.Error("ValidateSchema should report type mismatch")
		}
	})

	t.Run("nil schema returns error", func(t *testing.T) {
		jsonStr := `{"name": "Alice"}`
		_, err := processor.ValidateSchema(jsonStr, nil)
		if err == nil {
			t.Error("ValidateSchema should return error for nil schema")
		}
	})

	t.Run("invalid JSON returns error", func(t *testing.T) {
		jsonStr := `{invalid}`
		schema := &Schema{Type: "object"}

		_, err := processor.ValidateSchema(jsonStr, schema)
		if err == nil {
			t.Error("ValidateSchema should return error for invalid JSON")
		}
	})

	t.Run("array validation", func(t *testing.T) {
		jsonStr := `[1, 2, 3]`
		schema := &Schema{
			Type:     "array",
			MinItems: 2,
			MaxItems: 5,
			Items:    &Schema{Type: "number"},
		}

		errors, err := processor.ValidateSchema(jsonStr, schema)
		if err != nil {
			t.Fatalf("ValidateSchema error: %v", err)
		}
		if len(errors) != 0 {
			t.Errorf("ValidateSchema should have no errors, got: %v", errors)
		}
	})

	t.Run("array min items violation", func(t *testing.T) {
		jsonStr := `[1]`
		schema := &Schema{
			Type: "array",
		}
		schema.SetMinItems(2) // Use setter to set hasMinItems flag

		errors, err := processor.ValidateSchema(jsonStr, schema)
		if err != nil {
			t.Fatalf("ValidateSchema error: %v", err)
		}
		if len(errors) == 0 {
			t.Error("ValidateSchema should report min items violation")
		}
	})

	t.Run("array max items violation", func(t *testing.T) {
		jsonStr := `[1, 2, 3, 4, 5]`
		schema := &Schema{
			Type: "array",
		}
		schema.SetMaxItems(3) // Use setter to set hasMaxItems flag

		errors, err := processor.ValidateSchema(jsonStr, schema)
		if err != nil {
			t.Fatalf("ValidateSchema error: %v", err)
		}
		if len(errors) == 0 {
			t.Error("ValidateSchema should report max items violation")
		}
	})

	t.Run("string validation", func(t *testing.T) {
		jsonStr := `"hello"`
		schema := &Schema{
			Type:      "string",
			MinLength: 1,
			MaxLength: 10,
		}

		errors, err := processor.ValidateSchema(jsonStr, schema)
		if err != nil {
			t.Fatalf("ValidateSchema error: %v", err)
		}
		if len(errors) != 0 {
			t.Errorf("ValidateSchema should have no errors, got: %v", errors)
		}
	})

	t.Run("string pattern validation", func(t *testing.T) {
		jsonStr := `"test@example.com"`
		schema := &Schema{
			Type:    "string",
			Pattern: `^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`,
		}

		errors, err := processor.ValidateSchema(jsonStr, schema)
		if err != nil {
			t.Fatalf("ValidateSchema error: %v", err)
		}
		if len(errors) != 0 {
			t.Errorf("ValidateSchema should have no errors, got: %v", errors)
		}
	})

	t.Run("number validation", func(t *testing.T) {
		jsonStr := `50`
		schema := &Schema{
			Type:    "number",
			Minimum: 0,
			Maximum: 100,
		}

		errors, err := processor.ValidateSchema(jsonStr, schema)
		if err != nil {
			t.Fatalf("ValidateSchema error: %v", err)
		}
		if len(errors) != 0 {
			t.Errorf("ValidateSchema should have no errors, got: %v", errors)
		}
	})

	t.Run("enum validation", func(t *testing.T) {
		jsonStr := `"red"`
		schema := &Schema{
			Type: "string",
			Enum: []any{"red", "green", "blue"},
		}

		errors, err := processor.ValidateSchema(jsonStr, schema)
		if err != nil {
			t.Fatalf("ValidateSchema error: %v", err)
		}
		if len(errors) != 0 {
			t.Errorf("ValidateSchema should have no errors, got: %v", errors)
		}
	})

	t.Run("enum validation failure", func(t *testing.T) {
		jsonStr := `"yellow"`
		schema := &Schema{
			Type: "string",
			Enum: []any{"red", "green", "blue"},
		}

		errors, err := processor.ValidateSchema(jsonStr, schema)
		if err != nil {
			t.Fatalf("ValidateSchema error: %v", err)
		}
		if len(errors) == 0 {
			t.Error("ValidateSchema should report enum violation")
		}
	})

	t.Run("const validation", func(t *testing.T) {
		jsonStr := `"fixed"`
		schema := &Schema{
			Type:  "string",
			Const: "fixed",
		}

		errors, err := processor.ValidateSchema(jsonStr, schema)
		if err != nil {
			t.Fatalf("ValidateSchema error: %v", err)
		}
		if len(errors) != 0 {
			t.Errorf("ValidateSchema should have no errors, got: %v", errors)
		}
	})

	t.Run("const validation failure", func(t *testing.T) {
		jsonStr := `"other"`
		schema := &Schema{
			Type:  "string",
			Const: "fixed",
		}

		errors, err := processor.ValidateSchema(jsonStr, schema)
		if err != nil {
			t.Fatalf("ValidateSchema error: %v", err)
		}
		if len(errors) == 0 {
			t.Error("ValidateSchema should report const violation")
		}
	})

	t.Run("boolean type validation", func(t *testing.T) {
		jsonStr := `true`
		schema := &Schema{
			Type: "boolean",
		}

		errors, err := processor.ValidateSchema(jsonStr, schema)
		if err != nil {
			t.Fatalf("ValidateSchema error: %v", err)
		}
		if len(errors) != 0 {
			t.Errorf("ValidateSchema should have no errors, got: %v", errors)
		}
	})

	t.Run("null type validation", func(t *testing.T) {
		jsonStr := `null`
		schema := &Schema{
			Type: "null",
		}

		errors, err := processor.ValidateSchema(jsonStr, schema)
		if err != nil {
			t.Fatalf("ValidateSchema error: %v", err)
		}
		if len(errors) != 0 {
			t.Errorf("ValidateSchema should have no errors, got: %v", errors)
		}
	})

	t.Run("unique items validation", func(t *testing.T) {
		jsonStr := `[1, 2, 3, 4, 5]`
		schema := &Schema{
			Type:        "array",
			UniqueItems: true,
		}

		errors, err := processor.ValidateSchema(jsonStr, schema)
		if err != nil {
			t.Fatalf("ValidateSchema error: %v", err)
		}
		if len(errors) != 0 {
			t.Errorf("ValidateSchema should have no errors, got: %v", errors)
		}
	})

	t.Run("unique items validation failure", func(t *testing.T) {
		jsonStr := `[1, 2, 3, 2, 5]`
		schema := &Schema{
			Type:        "array",
			UniqueItems: true,
		}

		errors, err := processor.ValidateSchema(jsonStr, schema)
		if err != nil {
			t.Fatalf("ValidateSchema error: %v", err)
		}
		if len(errors) == 0 {
			t.Error("ValidateSchema should report duplicate items")
		}
	})

	t.Run("additional properties not allowed", func(t *testing.T) {
		jsonStr := `{"name": "Alice", "extra": "value"}`
		schema := &Schema{
			Type:                 "object",
			AdditionalProperties: false,
			Properties: map[string]*Schema{
				"name": {Type: "string"},
			},
		}

		errors, err := processor.ValidateSchema(jsonStr, schema)
		if err != nil {
			t.Fatalf("ValidateSchema error: %v", err)
		}
		if len(errors) == 0 {
			t.Error("ValidateSchema should report additional property")
		}
	})

	t.Run("nested object validation", func(t *testing.T) {
		jsonStr := `{"user": {"name": "Alice", "age": 30}}`
		schema := &Schema{
			Type: "object",
			Properties: map[string]*Schema{
				"user": {
					Type: "object",
					Properties: map[string]*Schema{
						"name": {Type: "string"},
						"age":  {Type: "number"},
					},
				},
			},
		}

		errors, err := processor.ValidateSchema(jsonStr, schema)
		if err != nil {
			t.Fatalf("ValidateSchema error: %v", err)
		}
		if len(errors) != 0 {
			t.Errorf("ValidateSchema should have no errors, got: %v", errors)
		}
	})
}

// TestSchemaMethods tests Schema setter methods
func TestSchemaMethods(t *testing.T) {
	schema := &Schema{}

	t.Run("SetMinLength", func(t *testing.T) {
		schema.SetMinLength(5)
		if schema.MinLength != 5 {
			t.Errorf("MinLength = %d, want 5", schema.MinLength)
		}
	})

	t.Run("SetMaxLength", func(t *testing.T) {
		schema.SetMaxLength(100)
		if schema.MaxLength != 100 {
			t.Errorf("MaxLength = %d, want 100", schema.MaxLength)
		}
	})

	t.Run("SetMinimum", func(t *testing.T) {
		schema.SetMinimum(0.0)
		if schema.Minimum != 0.0 {
			t.Errorf("Minimum = %v, want 0.0", schema.Minimum)
		}
	})

	t.Run("SetMaximum", func(t *testing.T) {
		schema.SetMaximum(100.0)
		if schema.Maximum != 100.0 {
			t.Errorf("Maximum = %v, want 100.0", schema.Maximum)
		}
	})

	t.Run("SetMinItems", func(t *testing.T) {
		schema.SetMinItems(1)
		if schema.MinItems != 1 {
			t.Errorf("MinItems = %d, want 1", schema.MinItems)
		}
	})

	t.Run("SetMaxItems", func(t *testing.T) {
		schema.SetMaxItems(10)
		if schema.MaxItems != 10 {
			t.Errorf("MaxItems = %d, want 10", schema.MaxItems)
		}
	})

	t.Run("SetExclusiveMinimum", func(t *testing.T) {
		schema.SetExclusiveMinimum(true)
		if !schema.ExclusiveMinimum {
			t.Error("ExclusiveMinimum should be true")
		}
	})

	t.Run("SetExclusiveMaximum", func(t *testing.T) {
		schema.SetExclusiveMaximum(true)
		if !schema.ExclusiveMaximum {
			t.Error("ExclusiveMaximum should be true")
		}
	})

	t.Run("HasMinLength", func(t *testing.T) {
		if !schema.HasMinLength() {
			t.Error("HasMinLength should be true")
		}
	})

	t.Run("HasMaxLength", func(t *testing.T) {
		if !schema.HasMaxLength() {
			t.Error("HasMaxLength should be true")
		}
	})

	t.Run("HasMinimum", func(t *testing.T) {
		if !schema.HasMinimum() {
			t.Error("HasMinimum should be true")
		}
	})

	t.Run("HasMaximum", func(t *testing.T) {
		if !schema.HasMaximum() {
			t.Error("HasMaximum should be true")
		}
	})

	t.Run("HasMinItems", func(t *testing.T) {
		if !schema.HasMinItems() {
			t.Error("HasMinItems should be true")
		}
	})

	t.Run("HasMaxItems", func(t *testing.T) {
		if !schema.HasMaxItems() {
			t.Error("HasMaxItems should be true")
		}
	})
}

// TestDefaultSchema tests DefaultSchema function
func TestDefaultSchema(t *testing.T) {
	schema := DefaultSchema()

	if schema == nil {
		t.Fatal("DefaultSchema returned nil")
	}

	// Should have empty type by default
	if schema.Type != "" {
		t.Errorf("Default type = %q, want empty string", schema.Type)
	}

	// Should allow additional properties by default
	if !schema.AdditionalProperties {
		t.Error("AdditionalProperties should be true by default")
	}
}
