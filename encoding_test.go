package json

import (
	"bytes"
	"encoding/json"
	"errors"
	"math"
	"strings"
	"testing"
	"time"
)

// TestFloatEncoding_MatchesStdlib is a golden test for [D-002] C1: this package's
// float formatting must match encoding/json byte-for-byte, including the 1e21
// threshold (scientific notation) and the very-small (<1e-6) scientific form.
// Previously the fast encoder used 'f' unconditionally, emitting a 22-digit
// decimal for 1e21; the custom encoder used 'g' (uppercase E). Both now route
// through internal.AppendJSONFloat which mirrors stdlib's floatEncoder.
func TestFloatEncoding_MatchesStdlib(t *testing.T) {
	values := []float64{
		0, 1, -1, 100, -100,
		0.5, -0.5, 3.141592653589793,
		1e6, 1e15, 1e18, 1e20, // 'f' range
		1e21, 1e22, 6.022e23, // 'e' range (>= 1e21)
		1e-6, 1e-7, 9e-7, 1e-10, 1.5e-300, // 'e' range (< 1e-6)
		123456789.123456789,
		math.Copysign(0, -1), // negative zero -> "-0"
	}
	for _, v := range values {
		got, err := Marshal(v)
		if err != nil {
			t.Errorf("Marshal(%v) error: %v", v, err)
			continue
		}
		want, _ := json.Marshal(v)
		if string(got) != string(want) {
			t.Errorf("Marshal(%v) = %q, stdlib = %q (mismatch)", v, got, want)
		}
	}
	// float32 through the fast path as well.
	for _, v := range []float32{1e6, 1e21, 1e-7, 3.14} {
		got, err := Marshal(v)
		if err != nil {
			t.Errorf("Marshal(float32 %v) error: %v", v, err)
			continue
		}
		want, _ := json.Marshal(v)
		if string(got) != string(want) {
			t.Errorf("Marshal(float32 %v) = %q, stdlib = %q (mismatch)", v, got, want)
		}
	}
}

// TestEncodingAdvanced tests advanced encoding features
func TestEncodingAdvanced(t *testing.T) {
	helper := newTestHelper(t)

	t.Run("EncodeWithConfig", func(t *testing.T) {
		processor, _ := New(DefaultConfig())
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
			config := DefaultConfig()
			config.Pretty = true
			config.Indent = "  "

			result, err := processor.EncodeWithConfig(user, config)
			helper.AssertNoError(err)
			helper.AssertTrue(strings.Contains(result, "\n"))
			helper.AssertTrue(strings.Contains(result, "  "))
		})

		t.Run("CompactEncoding", func(t *testing.T) {
			config := DefaultConfig()
			config.Pretty = false

			result, err := processor.EncodeWithConfig(user, config)
			helper.AssertNoError(err)
			helper.AssertFalse(strings.Contains(result, "\n"))
		})

		t.Run("SortKeys", func(t *testing.T) {
			config := DefaultConfig()
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

			encodeConfig := DefaultConfig()
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
				config := DefaultConfig()
				config.FloatPrecision = 2

				result, err := processor.EncodeWithConfig(data, config)
				helper.AssertNoError(err)
				helper.AssertTrue(strings.Contains(result, "3.14"))
			})

			t.Run("Precision4", func(t *testing.T) {
				config := DefaultConfig()
				config.FloatPrecision = 4

				result, err := processor.EncodeWithConfig(data, config)
				helper.AssertNoError(err)
				// Check that result contains some form of the number
				helper.AssertTrue(strings.Contains(result, "3.1"))
			})
		})

		t.Run("FloatTruncate", func(t *testing.T) {
			type Data struct {
				Value float64 `json:"value"`
			}

			tests := []struct {
				name           string
				value          float64
				precision      int
				truncate       bool
				expectedSubstr string
			}{
				{"TruncateVsRound_Pi_Truncate", 3.141592653589793, 4, true, "3.1415"},
				{"TruncateVsRound_Pi_Round", 3.141592653589793, 4, false, "3.1416"},
				{"Truncate_Precision2", 3.149, 2, true, "3.14"},
				{"Truncate_NegativeNumber", -3.141592653589793, 4, true, "-3.1415"},
				{"Truncate_Precision0", 3.999, 0, true, ":3"},
				{"Truncate_SmallNumber", 0.123456789, 3, true, "0.123"},
				{"Truncate_LargePrecision", 1.5, 5, true, "1.50000"},
				{"Truncate_IntegerValue", 42.0, 3, true, "42.000"},
				{"Truncate_VerySmallNumber", 0.00000123456, 8, true, "0.00000123"},
			}

			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					data := Data{Value: tt.value}
					config := DefaultConfig()
					config.FloatPrecision = tt.precision
					config.FloatTruncate = tt.truncate

					result, err := processor.EncodeWithConfig(data, config)
					helper.AssertNoError(err)
					helper.AssertTrue(strings.Contains(result, tt.expectedSubstr),
						"Expected %q in result, got %q", tt.expectedSubstr, result)
				})
			}
		})
	})

	t.Run("HTMLEscaping", func(t *testing.T) {
		processor, _ := New(DefaultConfig())
		defer processor.Close()

		type Data struct {
			HTML string `json:"html"`
		}

		data := Data{HTML: `<script>alert("XSS")</script>`}

		t.Run("EscapeHTML", func(t *testing.T) {
			config := DefaultConfig()
			config.EscapeHTML = true

			result, err := processor.EncodeWithConfig(data, config)
			helper.AssertNoError(err)

			// Should be escaped as unicode
			helper.AssertTrue(strings.Contains(result, "\\u003c"))
		})

		t.Run("NoEscapeHTML", func(t *testing.T) {
			config := DefaultConfig()
			config.EscapeHTML = false

			result, err := processor.EncodeWithConfig(data, config)
			helper.AssertNoError(err)
			helper.AssertTrue(strings.Contains(result, "<script>"))
			helper.AssertFalse(strings.Contains(result, "\\u003c"))
		})
	})

	t.Run("UnicodeEscaping", func(t *testing.T) {
		processor, _ := New(DefaultConfig())
		defer processor.Close()

		type Data struct {
			Text string `json:"text"`
		}

		data := Data{Text: "Hello 世界 🌍"}

		t.Run("EscapeUnicode", func(t *testing.T) {
			config := DefaultConfig()
			config.EscapeUnicode = true

			result, err := processor.EncodeWithConfig(data, config)
			helper.AssertNoError(err)
			// Non-ASCII must not survive as raw UTF-8.
			helper.AssertFalse(strings.Contains(result, "世界"))
		})

		t.Run("NoEscapeUnicode", func(t *testing.T) {
			config := DefaultConfig()
			config.EscapeUnicode = false

			result, err := processor.EncodeWithConfig(data, config)
			helper.AssertNoError(err)

			// Unicode should be preserved
			helper.AssertTrue(strings.Contains(result, "世界"))
		})
	})

	t.Run("IncludeNulls", func(t *testing.T) {
		processor, _ := New(DefaultConfig())
		defer processor.Close()

		type Data struct {
			Name  string `json:"name"`
			Email string `json:"email"`
			Extra any    `json:"extra"`
		}

		data := Data{
			Name:  "John",
			Extra: nil,
		}

		t.Run("IncludeNullsTrue", func(t *testing.T) {
			config := DefaultConfig()
			config.IncludeNulls = true
			config.Pretty = true

			result, err := processor.EncodeWithConfig(data, config)
			helper.AssertNoError(err)
			helper.AssertTrue(strings.Contains(result, "\"name\""))
			// Nil any field must be emitted as an explicit null.
			helper.AssertTrue(strings.Contains(result, `"extra": null`))
		})

		t.Run("IncludeNullsFalse", func(t *testing.T) {
			config := DefaultConfig()
			config.IncludeNulls = false
			config.Pretty = true

			result, err := processor.EncodeWithConfig(data, config)
			helper.AssertNoError(err)
			helper.AssertTrue(strings.Contains(result, "\"name\""))
			// Nil any field must be omitted; empty strings are unaffected.
			helper.AssertFalse(strings.Contains(result, "\"extra\""))
			helper.AssertTrue(strings.Contains(result, `"email": ""`))
		})
	})
}

// TestEncodingTypes tests encoding of various Go types
func TestEncodingTypes(t *testing.T) {
	helper := newTestHelper(t)

	processor, _ := New(DefaultConfig())
	defer processor.Close()

	t.Run("BasicTypes", func(t *testing.T) {
		// Scalar encoding is byte-exact; assert the full output, not substrings
		// (the former "42" substring check would pass for "42x" or "142").
		tests := []struct {
			name  string
			value interface{}
			want  string
		}{
			{"String", "hello", `"hello"`},
			{"Int", 42, `42`},
			{"Float", 3.14, `3.14`},
			{"Bool", true, `true`},
			{"Nil", nil, `null`},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				result, err := processor.EncodeWithConfig(tt.value, DefaultConfig())
				helper.AssertNoError(err)
				helper.AssertEqual(tt.want, result)
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

			result, err := processor.EncodeWithConfig(data, DefaultConfig())
			helper.AssertNoError(err)
			helper.AssertTrue(strings.Contains(result, "\"name\""))
			helper.AssertTrue(strings.Contains(result, "\"age\""))
			helper.AssertTrue(strings.Contains(result, "\"admin\""))
		})

		t.Run("Slice", func(t *testing.T) {
			data := []interface{}{1, "two", 3.0, true}

			result, err := processor.EncodeWithConfig(data, DefaultConfig())
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

			result, err := processor.EncodeWithConfig(user, DefaultConfig())
			helper.AssertNoError(err)
			helper.AssertTrue(strings.Contains(result, "\"name\""))
			helper.AssertTrue(strings.Contains(result, "\"address\""))
			helper.AssertTrue(strings.Contains(result, "\"city\""))
		})
	})

	t.Run("Time", func(t *testing.T) {
		now := time.Now()

		result, err := processor.EncodeWithConfig(now, DefaultConfig())
		helper.AssertNoError(err)

		// Should be RFC3339 format
		parsed := &time.Time{}
		err = json.Unmarshal([]byte(result), parsed)
		helper.AssertNoError(err)
	})
}

// TestEncodingStreams tests stream encoding
func TestEncodingStreams(t *testing.T) {
	helper := newTestHelper(t)

	processor, _ := New(DefaultConfig())
	defer processor.Close()

	t.Run("EncodeStream", func(t *testing.T) {
		data := []interface{}{
			map[string]interface{}{"id": 1, "name": "Item 1"},
			map[string]interface{}{"id": 2, "name": "Item 2"},
			map[string]interface{}{"id": 3, "name": "Item 3"},
		}

		t.Run("Pretty", func(t *testing.T) {
			cfg := DefaultConfig()
			cfg.Pretty = true
			result, err := processor.EncodeStream(data, cfg)
			helper.AssertNoError(err)
			helper.AssertTrue(strings.Contains(result, "\n"))
		})

		t.Run("Compact", func(t *testing.T) {
			cfg := DefaultConfig()
			cfg.Pretty = false
			result, err := processor.EncodeStream(data, cfg)
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

		cfg := DefaultConfig()
		cfg.Pretty = true
		result, err := processor.EncodeBatch(pairs, cfg)
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

		cfg := DefaultConfig()
		cfg.Pretty = true
		result, err := processor.EncodeFields(user, fields, cfg)
		helper.AssertNoError(err)

		helper.AssertTrue(strings.Contains(result, "\"id\""))
		helper.AssertTrue(strings.Contains(result, "\"name\""))
		helper.AssertFalse(strings.Contains(result, "\"email\""))
		helper.AssertFalse(strings.Contains(result, "\"password\""))
	})
}

// TestEncodingCompatibility tests compatibility with encoding/json
func TestEncodingCompatibility(t *testing.T) {
	helper := newTestHelper(t)

	t.Run("MarshalUnmarshal", func(t *testing.T) {
		type User struct {
			Name string `json:"name"`
			Age  int    `json:"age"`
		}

		original := User{Name: "Alice", Age: 30}

		// Marshal with our processor
		processor, _ := New(DefaultConfig())
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

		processor, _ := New(DefaultConfig())
		defer processor.Close()

		// Encode
		encoded, err := processor.EncodeWithConfig(data, DefaultConfig())
		helper.AssertNoError(err)

		// Decode
		var decoded map[string]interface{}
		err = processor.Unmarshal([]byte(encoded), &decoded)
		helper.AssertNoError(err)

		// Verify
		helper.AssertEqual(data["string"], decoded["string"])
		helper.AssertEqual(data["bool"], decoded["bool"])
		helper.AssertNil(decoded["null"])
	})
}

// TestEncodingErrors tests encoding error conditions
func TestEncodingErrors(t *testing.T) {
	helper := newTestHelper(t)

	processor, _ := New(DefaultConfig())
	defer processor.Close()

	t.Run("ClosedProcessor", func(t *testing.T) {
		processor.Close()

		data := map[string]string{"key": "value"}
		_, err := processor.EncodeWithConfig(data, DefaultConfig())
		helper.AssertError(err)
	})

	t.Run("DepthLimit", func(t *testing.T) {
		config := DefaultConfig()
		config.MaxDepth = 2

		// Create deeply nested structure
		deepData := map[string]interface{}{
			"level1": map[string]interface{}{
				"level2": map[string]interface{}{
					"level3": "too deep",
				},
			},
		}

		_, err := processor.EncodeWithConfig(deepData, config)
		helper.AssertError(err)
	})

	t.Run("InvalidType", func(t *testing.T) {
		// Try to encode a channel (unsupported)
		ch := make(chan int)
		defer close(ch)

		_, err := processor.EncodeWithConfig(ch, DefaultConfig())
		helper.AssertError(err)
	})
}

// TestEncodeDecodeIntegration tests encode/decode integration
func TestEncodeDecodeIntegration(t *testing.T) {
	helper := newTestHelper(t)

	processor, _ := New(DefaultConfig())
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
		encoded, err := processor.EncodeWithConfig(original, DefaultConfig())
		helper.AssertNoError(err)

		// Decode
		var decoded User
		err = processor.Unmarshal([]byte(encoded), &decoded)
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
		err = processor.Unmarshal(buf.Bytes(), &decoded)
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
	processor, _ := New()
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
			Type:     "array",
			MinItems: 2,
		}
		schema.hasMinItems = true

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
			Type:     "array",
			MaxItems: 3,
		}
		schema.hasMaxItems = true

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

// ============================================================================
// Additional encoding coverage tests
// ============================================================================

// TestEncoding_ParseBooleanAndNull tests Decoder parseBoolean/parseNull paths
func TestEncoding_ParseBooleanAndNull(t *testing.T) {
	t.Run("boolean true", func(t *testing.T) {
		dec := NewDecoder(strings.NewReader("true"))
		tok, err := dec.Token()
		if err != nil {
			t.Fatalf("Token() error: %v", err)
		}
		if tok != true {
			t.Errorf("Token() = %v, want true", tok)
		}
	})

	t.Run("boolean false", func(t *testing.T) {
		dec := NewDecoder(strings.NewReader("false"))
		tok, err := dec.Token()
		if err != nil {
			t.Fatalf("Token() error: %v", err)
		}
		if tok != false {
			t.Errorf("Token() = %v, want false", tok)
		}
	})

	t.Run("null", func(t *testing.T) {
		dec := NewDecoder(strings.NewReader("null"))
		tok, err := dec.Token()
		if err != nil {
			t.Fatalf("Token() error: %v", err)
		}
		if tok != nil {
			t.Errorf("Token() = %v, want nil", tok)
		}
	})

	t.Run("invalid boolean", func(t *testing.T) {
		dec := NewDecoder(strings.NewReader("trx"))
		_, err := dec.Token()
		if err == nil {
			t.Error("expected error for invalid boolean")
		}
	})

	t.Run("invalid null", func(t *testing.T) {
		dec := NewDecoder(strings.NewReader("nulx"))
		_, err := dec.Token()
		if err == nil {
			t.Error("expected error for invalid null")
		}
	})
}

// TestEncoding_ParseStringEscapes tests parseString escape sequence handling
func TestEncoding_ParseStringEscapes(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"backspace", `"a\bc"`, "a\bc"},
		{"formfeed", `"a\fc"`, "a\fc"},
		{"newline", `"a\nc"`, "a\nc"},
		{"carriage return", `"a\rc"`, "a\rc"},
		{"tab", `"a\tc"`, "a\tc"},
		{"escaped quote", `"a\"c"`, "a\"c"},
		{"escaped backslash", `"a\\c"`, "a\\c"},
		{"escaped slash", `"a\/c"`, "a/c"},
		{"unicode escape", `"a\u0041c"`, "aAc"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dec := NewDecoder(strings.NewReader(tt.input))
			var result string
			err := dec.Decode(&result)
			if err != nil {
				t.Fatalf("Decode() error: %v", err)
			}
			if result != tt.want {
				t.Errorf("Decode(%q) = %q, want %q", tt.input, result, tt.want)
			}
		})
	}
}

// --- Encoding with config variants ---

func TestEncodeStructConfigVariants(t *testing.T) {
	type dataStruct struct {
		URL    string  `json:"url"`
		Score  float64 `json:"score"`
		Active bool    `json:"active"`
	}

	t.Run("escape unicode", func(t *testing.T) {
		s := dataStruct{URL: "hello世界", Score: 1.0, Active: true}
		cfg := DefaultConfig()
		cfg.EscapeUnicode = true
		result, err := Encode(s, cfg)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(result, "\\u") {
			t.Errorf("expected unicode escaping in result: %s", result)
		}
	})

	t.Run("escape slash", func(t *testing.T) {
		s := dataStruct{URL: "http://example.com/path", Score: 1.0, Active: true}
		cfg := DefaultConfig()
		cfg.EscapeSlash = true
		result, err := Encode(s, cfg)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(result, "\\/") {
			t.Errorf("expected slash escaping in result: %s", result)
		}
	})

	t.Run("escape newlines and tabs disabled", func(t *testing.T) {
		type tabStruct struct {
			Text string `json:"text"`
		}
		s := tabStruct{Text: "line1\nline2\ttab"}
		cfg := DefaultConfig()
		cfg.EscapeNewlines = false
		cfg.EscapeTabs = false
		result, err := Encode(s, cfg)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(result, "line1\nline2") {
			t.Errorf("expected literal newline: %q", result)
		}
	})
}

func TestEncodeStructCustomAllFlags(t *testing.T) {
	type fullStruct struct {
		Name   string  `json:"name"`
		Null   *string `json:"null,omitempty"`
		HTML   string  `json:"html"`
		Number float64 `json:"number"`
	}

	s := fullStruct{Name: "test", Null: nil, HTML: "<b>", Number: 1.23456}
	cfg := DefaultConfig()
	cfg.IncludeNulls = false
	cfg.SortKeys = true
	cfg.EscapeHTML = false
	cfg.FloatPrecision = 2
	cfg.EscapeNewlines = false
	cfg.EscapeTabs = false
	result, err := Encode(s, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, `"name"`) {
		t.Errorf("expected name field in result: %s", result)
	}
	if strings.Contains(result, "null") {
		t.Errorf("expected null field excluded: %s", result)
	}
}

func TestEncodeValueTypes(t *testing.T) {
	cfg := DefaultConfig()
	cfg.SortKeys = true

	tests := []struct {
		name string
		val  any
		want string
	}{
		{name: "nil value", val: nil, want: "null"},
		{name: "bool true", val: true, want: "true"},
		{name: "bool false", val: false, want: "false"},
		{name: "int value", val: 42, want: "42"},
		{name: "float value", val: 3.14, want: "3.14"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := Encode(tt.val, cfg)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			assertJSONEqual(t, tt.want, result)
		})
	}
}

// TestEncoding_ParseStringErrors tests parseString error paths
func TestEncoding_ParseStringErrors(t *testing.T) {
	t.Run("invalid escape", func(t *testing.T) {
		dec := NewDecoder(strings.NewReader(`"a\x"`))
		var s string
		err := dec.Decode(&s)
		if err == nil {
			t.Error("expected error for invalid escape sequence")
		}
	})

	t.Run("truncated string", func(t *testing.T) {
		dec := NewDecoder(strings.NewReader(`"unclosed`))
		var s string
		err := dec.Decode(&s)
		if err == nil {
			t.Error("expected error for truncated string")
		}
	})

	t.Run("truncated unicode escape", func(t *testing.T) {
		dec := NewDecoder(strings.NewReader(`"\u00"`))
		var s string
		err := dec.Decode(&s)
		if err == nil {
			t.Error("expected error for truncated unicode escape")
		}
	})
}

// TestEncoding_NewEncoder tests NewEncoder with various configs
func TestEncoding_NewEncoder(t *testing.T) {
	t.Run("default config", func(t *testing.T) {
		var buf bytes.Buffer
		enc := NewEncoder(&buf)
		if enc == nil {
			t.Fatal("NewEncoder returned nil")
		}
	})

	t.Run("pretty config", func(t *testing.T) {
		var buf bytes.Buffer
		cfg := DefaultConfig()
		cfg.Pretty = true
		enc := NewEncoder(&buf, cfg)
		err := enc.Encode(map[string]any{"a": 1})
		if err != nil {
			t.Fatalf("Encode error: %v", err)
		}
		if !bytes.Contains(buf.Bytes(), []byte("\n")) {
			t.Error("pretty output should contain newlines")
		}
	})

	t.Run("no escape HTML", func(t *testing.T) {
		var buf bytes.Buffer
		cfg := DefaultConfig()
		cfg.EscapeHTML = false
		enc := NewEncoder(&buf, cfg)
		err := enc.Encode(map[string]any{"html": "<script>"})
		if err != nil {
			t.Fatalf("Encode error: %v", err)
		}
		if bytes.Contains(buf.Bytes(), []byte(`\u003c`)) {
			t.Error("should not escape HTML when EscapeHTML=false")
		}
	})
}

// TestEncoding_EncodeStruct tests encodeStruct code paths
func TestEncoding_EncodeStruct(t *testing.T) {
	type testStruct struct {
		Name   string `json:"name"`
		Value  int    `json:"value"`
		NilVal any    `json:"nil_val,omitempty"`
	}

	t.Run("default config", func(t *testing.T) {
		p, _ := New()
		defer p.Close()
		result, err := p.EncodeWithConfig(testStruct{Name: "test", Value: 42}, DefaultConfig())
		if err != nil {
			t.Fatalf("EncodeWithConfig error: %v", err)
		}
		if !strings.Contains(result, `"name":"test"`) || !strings.Contains(result, `"value":42`) {
			t.Errorf("unexpected encode output: %s", result)
		}
	})

	t.Run("include nulls false", func(t *testing.T) {
		p, _ := New()
		defer p.Close()
		cfg := DefaultConfig()
		cfg.IncludeNulls = false
		result, err := p.EncodeWithConfig(testStruct{Name: "test", Value: 42}, cfg)
		if err != nil {
			t.Fatalf("EncodeWithConfig error: %v", err)
		}
		if !strings.Contains(result, `"name":"test"`) {
			t.Errorf("missing name field in output: %s", result)
		}
	})

	t.Run("sort keys", func(t *testing.T) {
		p, _ := New()
		defer p.Close()
		cfg := DefaultConfig()
		cfg.SortKeys = true
		result, err := p.EncodeWithConfig(map[string]any{"z": 1, "a": 2}, cfg)
		if err != nil {
			t.Fatalf("EncodeWithConfig error: %v", err)
		}
		// Sorted output must place "a" before "z".
		if strings.Index(result, `"a"`) > strings.Index(result, `"z"`) {
			t.Errorf("keys not sorted in output: %s", result)
		}
	})

	t.Run("pretty", func(t *testing.T) {
		p, _ := New()
		defer p.Close()
		cfg := DefaultConfig()
		cfg.Pretty = true
		result, err := p.EncodeWithConfig(testStruct{Name: "test", Value: 42}, cfg)
		if err != nil {
			t.Fatalf("EncodeWithConfig error: %v", err)
		}
		if !strings.Contains(result, "\n") {
			t.Errorf("pretty output should be multi-line: %s", result)
		}
	})
}

// ---------------------------------------------------------------------------
// FIX-001: encoding boundary & error-path coverage
// ---------------------------------------------------------------------------

// TestEncode_DeepNesting_MaxDepthLimit exercises the pre-encode depth guard
// (encoding.go validateDepth). Marshal with Config.MaxDepth set must reject map
// data nested beyond the limit rather than encoding it.
func TestEncode_DeepNesting_MaxDepthLimit(t *testing.T) {
	// 5-level deep map[string]any: a -> a -> a -> a -> v
	deepest := map[string]any{"v": 1}
	cur := any(deepest)
	for i := 0; i < 4; i++ {
		cur = map[string]any{"a": cur}
	}
	cfg := DefaultConfig()
	cfg.MaxDepth = 3
	cfg.SortKeys = true // bypass the simple-type fast path so validateDepth runs
	_, err := Marshal(cur, cfg)
	if err == nil {
		t.Fatal("Marshal of data nested beyond MaxDepth must fail")
	}
	if !strings.Contains(err.Error(), "exceeds maximum") {
		t.Errorf("expected depth-limit error, got %v", err)
	}
}

// TestEncode_DeepStructNesting_ErrDepthLimit exercises the incremental depth
// check inside the custom encoder (encoding.go:1319). validateDepth does not
// recurse into structs, so deeply nested structs reach encodeValue's own guard
// and surface ErrDepthLimit.
func TestEncode_DeepStructNesting_ErrDepthLimit(t *testing.T) {
	type node struct {
		V any `json:"v"`
	}
	// node(1) -> node(2) -> node(3) -> node(4) -> 1
	nested := node{V: node{V: node{V: node{V: 1}}}}
	cfg := DefaultConfig()
	cfg.MaxDepth = 3
	cfg.SortKeys = true // force custom encoder so encodeValue's depth guard runs
	_, err := Encode(nested, cfg)
	if !errors.Is(err, ErrDepthLimit) {
		t.Fatalf("expected ErrDepthLimit, got %v", err)
	}
}

// TestEncodeJSONNumber_NonPreserve covers encodeJSONNumber's conversion branches
// (encoding.go:1419) used when PreserveNumbers is false: integer, float, and
// scientific-notation inputs must be normalized through Int64/Float64.
func TestEncodeJSONNumber_NonPreserve(t *testing.T) {
	cfg := DefaultConfig() // PreserveNumbers == false
	cfg.SortKeys = true    // force custom encoder -> encodeValue -> encodeJSONNumber
	cases := []struct {
		name string
		in   json.Number
		want string
	}{
		{"integer", json.Number("42"), "42"},
		{"negative int", json.Number("-7"), "-7"},
		{"float", json.Number("3.14"), "3.14"},
		{"scientific", json.Number("1e3"), "1000"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Encode(tc.in, cfg)
			if err != nil {
				t.Fatalf("Encode(%s): %v", tc.in, err)
			}
			if got != tc.want {
				t.Errorf("Encode(%s) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

// TestEncodeStruct_OmitEmpty_ZeroFields covers isEmpty's type branches
// (encoding.go:1847) via the custom struct encoder: zero-valued bool/int/uint/
// float/slice/string fields tagged omitempty must be dropped from output.
func TestEncodeStruct_OmitEmpty_ZeroFields(t *testing.T) {
	type tagged struct {
		Kept string  `json:"kept"`
		B    bool    `json:"b,omitempty"`
		I    int     `json:"i,omitempty"`
		U    uint    `json:"u,omitempty"`
		F    float64 `json:"f,omitempty"`
		S    string  `json:"s,omitempty"`
		Sl   []int   `json:"sl,omitempty"`
	}
	cfg := DefaultConfig()
	cfg.SortKeys = true // force custom encoder so isEmpty runs

	got, err := Encode(tagged{Kept: "x"}, cfg)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	// Only "kept" should survive; every zero omitempty field must be absent.
	var parsed map[string]any
	if err := json.Unmarshal([]byte(got), &parsed); err != nil {
		t.Fatalf("output not valid JSON: %v (out=%q)", err, got)
	}
	if len(parsed) != 1 || parsed["kept"] != "x" {
		t.Errorf("expected only {\"kept\":\"x\"}, got %q", got)
	}
}

// TestEncodeStructFields_Coverage exercises the field-iteration branches of
// encodeStructCustom (encoding.go:1747) that no other test reaches:
//   - json:"-" excludes the field entirely
//   - unexported fields are never emitted
//   - a json tag renames the field, and SortKeys orders by the renamed key
//   - a nil pointer is dropped when IncludeNulls=false and emitted as null when true
func TestEncodeStructFields_Coverage(t *testing.T) {
	type fields struct {
		Public   string  `json:"public"`
		Renamed  string  `json:"custom_name"`
		Hidden   string  `json:"-"`
		internal string  // unexported — must never appear in output
		Maybe    *string `json:"maybe"`
	}

	val := "set"
	populated := fields{
		Public:   "p",
		Renamed:  "r",
		Hidden:   "h",
		internal: "i",
		Maybe:    &val,
	}

	t.Run("rename, dash-tag, unexported excluded and SortKeys by tag", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.SortKeys = true // force custom encoder; also exercises tag-name sort

		got, err := Encode(populated, cfg)
		if err != nil {
			t.Fatalf("Encode: %v", err)
		}
		var parsed map[string]any
		if err := json.Unmarshal([]byte(got), &parsed); err != nil {
			t.Fatalf("invalid JSON: %v (out=%q)", err, got)
		}
		// json:"-" and the unexported field must be absent; the bare field name
		// "Renamed" must be replaced by its json tag "custom_name".
		for _, banned := range []string{"Hidden", "hidden", "internal", "Renamed"} {
			if _, ok := parsed[banned]; ok {
				t.Errorf("field %q should be excluded, got %q", banned, got)
			}
		}
		if parsed["custom_name"] != "r" {
			t.Errorf("rename to custom_name failed: %q", got)
		}
		if parsed["public"] != "p" {
			t.Errorf("public field lost: %q", got)
		}
		// SortKeys orders by key name: custom_name < maybe < public.
		if !strings.HasPrefix(got, `{"custom_name":`) {
			t.Errorf("expected custom_name first under SortKeys, got %q", got)
		}
	})

	t.Run("nil pointer excluded when IncludeNulls false", func(t *testing.T) {
		empty := fields{Public: "p", Maybe: nil}
		cfg := DefaultConfig()
		cfg.IncludeNulls = false
		cfg.SortKeys = true

		got, err := Encode(empty, cfg)
		if err != nil {
			t.Fatalf("Encode: %v", err)
		}
		if strings.Contains(got, "maybe") {
			t.Errorf("nil pointer should be dropped with IncludeNulls=false: %q", got)
		}
	})

	t.Run("nil pointer emitted as null when IncludeNulls true", func(t *testing.T) {
		empty := fields{Public: "p", Maybe: nil}
		cfg := DefaultConfig()
		cfg.IncludeNulls = true
		cfg.SortKeys = true

		got, err := Encode(empty, cfg)
		if err != nil {
			t.Fatalf("Encode: %v", err)
		}
		// With IncludeNulls=true the nil pointer is kept and renders as null.
		if !strings.Contains(got, `"maybe":null`) {
			t.Errorf("nil pointer should render as null with IncludeNulls=true: %q", got)
		}
	})
}
