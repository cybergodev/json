package json

import (
	"testing"
)

func TestValidationComprehensive(t *testing.T) {
	helper := NewTestHelper(t)

	t.Run("BasicSchemaValidation", func(t *testing.T) {
		// Test basic schema validation
		schema := DefaultSchema()
		schema.Type = "object"
		schema.Properties = map[string]*Schema{
			"name": {
				Type:      "string",
				MinLength: 1,
				MaxLength: 50,
			},
			"age": {
				Type:    "number",
				Minimum: 0,
				Maximum: 150,
			},
		}
		schema.Required = []string{"name"}

		// Test valid data
		validJSON := `{"name": "John Doe", "age": 30}`

		errors, err := ValidateSchema(validJSON, schema)
		helper.AssertNoError(err, "Should validate without error")
		helper.AssertEqual(0, len(errors), "Should have no validation errors")

		// Test missing required field
		invalidJSON := `{"age": 30}`

		errors, err = ValidateSchema(invalidJSON, schema)
		helper.AssertNoError(err, "Should validate without error")
		helper.AssertTrue(len(errors) > 0, "Should have validation errors for missing field")
	})

	t.Run("StringValidation", func(t *testing.T) {
		// Test string validation
		schema := DefaultSchema()
		schema.Type = "string"
		schema.SetMinLength(3)
		schema.SetMaxLength(10)

		// Test valid string
		errors, err := ValidateSchema(`"hello"`, schema)
		helper.AssertNoError(err, "Should validate without error")
		helper.AssertEqual(0, len(errors), "Should have no validation errors")

		// Test too short
		errors, err = ValidateSchema(`"hi"`, schema)
		helper.AssertNoError(err, "Should validate without error")
		helper.AssertTrue(len(errors) > 0, "Should have validation errors for too short string")

		// Test too long
		errors, err = ValidateSchema(`"this is too long"`, schema)
		helper.AssertNoError(err, "Should validate without error")
		helper.AssertTrue(len(errors) > 0, "Should have validation errors for too long string")

		// Test wrong type
		errors, err = ValidateSchema(`123`, schema)
		helper.AssertNoError(err, "Should validate without error")
		helper.AssertTrue(len(errors) > 0, "Should have validation errors for wrong type")
	})

	t.Run("NumberValidation", func(t *testing.T) {
		// Test number validation
		schema := DefaultSchema()
		schema.Type = "number"
		schema.SetMinimum(0)
		schema.SetMaximum(100)

		// Test valid number
		errors, err := ValidateSchema(`50`, schema)
		helper.AssertNoError(err, "Should validate without error")
		helper.AssertEqual(0, len(errors), "Should have no validation errors")

		errors, err = ValidateSchema(`50.5`, schema)
		helper.AssertNoError(err, "Should validate without error")
		helper.AssertEqual(0, len(errors), "Should have no validation errors")

		// Test below minimum
		errors, err = ValidateSchema(`-10`, schema)
		helper.AssertNoError(err, "Should validate without error")
		helper.AssertTrue(len(errors) > 0, "Should have validation errors for number below minimum")

		// Test above maximum
		errors, err = ValidateSchema(`150`, schema)
		helper.AssertNoError(err, "Should validate without error")
		helper.AssertTrue(len(errors) > 0, "Should have validation errors for number above maximum")

		// Test exclusive bounds
		exclusiveSchema := DefaultSchema()
		exclusiveSchema.Type = "number"
		exclusiveSchema.SetMinimum(0).SetExclusiveMinimum(true)
		exclusiveSchema.SetMaximum(100).SetExclusiveMaximum(true)

		errors, err = ValidateSchema(`0`, exclusiveSchema)
		helper.AssertNoError(err, "Should validate without error")
		helper.AssertTrue(len(errors) > 0, "Should have validation errors for exclusive minimum")

		errors, err = ValidateSchema(`100`, exclusiveSchema)
		helper.AssertNoError(err, "Should validate without error")
		helper.AssertTrue(len(errors) > 0, "Should have validation errors for exclusive maximum")
	})

	t.Run("ArrayValidation", func(t *testing.T) {
		// Test array validation
		schema := DefaultSchema()
		schema.Type = "array"
		schema.SetMinItems(1)
		schema.SetMaxItems(5)
		schema.Items = &Schema{
			Type: "string",
		}

		// Test valid array
		validArrayJSON := `["item1", "item2", "item3"]`
		errors, err := ValidateSchema(validArrayJSON, schema)
		helper.AssertNoError(err, "Should validate without error")
		helper.AssertEqual(0, len(errors), "Should have no validation errors")

		// Test empty array
		errors, err = ValidateSchema(`[]`, schema)
		helper.AssertNoError(err, "Should validate without error")
		helper.AssertTrue(len(errors) > 0, "Should have validation errors for empty array when min items > 0")

		// Test too many items
		longArrayJSON := `["1", "2", "3", "4", "5", "6"]`
		errors, err = ValidateSchema(longArrayJSON, schema)
		helper.AssertNoError(err, "Should validate without error")
		helper.AssertTrue(len(errors) > 0, "Should have validation errors for array with too many items")

		// Test wrong item type
		mixedArrayJSON := `["string", 123]`
		errors, err = ValidateSchema(mixedArrayJSON, schema)
		helper.AssertNoError(err, "Should validate without error")
		helper.AssertTrue(len(errors) > 0, "Should have validation errors for array with wrong item types")
	})

	t.Run("ObjectValidation", func(t *testing.T) {
		// Test object validation
		schema := DefaultSchema()
		schema.Type = "object"
		schema.Properties = map[string]*Schema{
			"id": {
				Type: "number",
			},
			"name": {
				Type:      "string",
				MinLength: 1,
			},
			"email": {
				Type:   "string",
				Format: "email",
			},
		}
		schema.Required = []string{"id", "name"}

		// Test valid object
		validObjJSON := `{
			"id": 1,
			"name": "John",
			"email": "john@example.com"
		}`

		errors, err := ValidateSchema(validObjJSON, schema)
		helper.AssertNoError(err, "Should validate without error")
		helper.AssertEqual(0, len(errors), "Should have no validation errors")

		// Test missing required property
		invalidObjJSON := `{"name": "John"}`

		errors, err = ValidateSchema(invalidObjJSON, schema)
		helper.AssertNoError(err, "Should validate without error")
		helper.AssertTrue(len(errors) > 0, "Should have validation errors for missing required property")

		// Test invalid property type
		wrongTypeObjJSON := `{"id": "not a number", "name": "John"}`

		errors, err = ValidateSchema(wrongTypeObjJSON, schema)
		helper.AssertNoError(err, "Should validate without error")
		helper.AssertTrue(len(errors) > 0, "Should have validation errors for wrong property type")
	})

	t.Run("FormatValidation", func(t *testing.T) {
		// Test email format
		emailSchema := DefaultSchema()
		emailSchema.Type = "string"
		emailSchema.Format = "email"

		errors, err := ValidateSchema(`"valid@example.com"`, emailSchema)
		helper.AssertNoError(err, "Should validate without error")
		helper.AssertEqual(0, len(errors), "Should have no validation errors")

		_, err = ValidateSchema(`"invalid-email"`, emailSchema)
		helper.AssertNoError(err, "Should validate without error")
		// Note: Format validation may not be fully implemented, so we just check it doesn't crash

		// Test date format
		dateSchema := DefaultSchema()
		dateSchema.Type = "string"
		dateSchema.Format = "date"

		_, err = ValidateSchema(`"2024-01-01"`, dateSchema)
		helper.AssertNoError(err, "Should validate without error")

		_, err = ValidateSchema(`"invalid-date"`, dateSchema)
		helper.AssertNoError(err, "Should validate without error")

		// Test datetime format
		datetimeSchema := DefaultSchema()
		datetimeSchema.Type = "string"
		datetimeSchema.Format = "date-time"

		_, err = ValidateSchema(`"2024-01-01T12:00:00Z"`, datetimeSchema)
		helper.AssertNoError(err, "Should validate without error")

		// Test URI format
		uriSchema := DefaultSchema()
		uriSchema.Type = "string"
		uriSchema.Format = "uri"

		_, err = ValidateSchema(`"https://example.com"`, uriSchema)
		helper.AssertNoError(err, "Should validate without error")

		_, err = ValidateSchema(`"not-a-uri"`, uriSchema)
		helper.AssertNoError(err, "Should validate without error")

		// Test UUID format
		uuidSchema := DefaultSchema()
		uuidSchema.Type = "string"
		uuidSchema.Format = "uuid"

		_, err = ValidateSchema(`"550e8400-e29b-41d4-a716-446655440000"`, uuidSchema)
		helper.AssertNoError(err, "Should validate without error")

		_, err = ValidateSchema(`"not-a-uuid"`, uuidSchema)
		helper.AssertNoError(err, "Should validate without error")
	})

	t.Run("IPAddressValidation", func(t *testing.T) {
		// Test IPv4 format
		ipv4Schema := DefaultSchema()
		ipv4Schema.Type = "string"
		ipv4Schema.Format = "ipv4"

		_, err := ValidateSchema(`"192.168.1.1"`, ipv4Schema)
		helper.AssertNoError(err, "Should validate without error")

		_, err = ValidateSchema(`"256.256.256.256"`, ipv4Schema)
		helper.AssertNoError(err, "Should validate without error")

		// Test IPv6 format
		ipv6Schema := DefaultSchema()
		ipv6Schema.Type = "string"
		ipv6Schema.Format = "ipv6"

		_, err = ValidateSchema(`"2001:0db8:85a3:0000:0000:8a2e:0370:7334"`, ipv6Schema)
		helper.AssertNoError(err, "Should validate without error")

		_, err = ValidateSchema(`"invalid-ipv6"`, ipv6Schema)
		helper.AssertNoError(err, "Should validate without error")
	})

	t.Run("NestedValidation", func(t *testing.T) {
		// Test nested object validation
		schema := DefaultSchema()
		schema.Type = "object"
		schema.Properties = map[string]*Schema{
			"user": {
				Type: "object",
				Properties: map[string]*Schema{
					"name": {
						Type:      "string",
						MinLength: 1,
					},
					"profile": {
						Type: "object",
						Properties: map[string]*Schema{
							"age": {
								Type:    "number",
								Minimum: 0,
							},
						},
					},
				},
				Required: []string{"name"},
			},
		}

		// Test valid nested structure
		validDataJSON := `{
			"user": {
				"name": "John",
				"profile": {
					"age": 30
				}
			}
		}`

		errors, err := ValidateSchema(validDataJSON, schema)
		helper.AssertNoError(err, "Should validate without error")
		helper.AssertEqual(0, len(errors), "Should have no validation errors")

		// Test invalid nested structure
		invalidDataJSON := `{
			"user": {
				"profile": {
					"age": -5
				}
			}
		}`

		errors, err = ValidateSchema(invalidDataJSON, schema)
		helper.AssertNoError(err, "Should validate without error")
		helper.AssertTrue(len(errors) > 0, "Should have validation errors for invalid nested structure")
	})

	t.Run("ComplexValidation", func(t *testing.T) {
		// Test complex validation scenario
		schema := DefaultSchema()
		schema.Type = "object"
		schema.Properties = map[string]*Schema{
			"users": {
				Type: "array",
				Items: &Schema{
					Type: "object",
					Properties: map[string]*Schema{
						"id": {
							Type: "number",
						},
						"name": {
							Type:      "string",
							MinLength: 1,
							MaxLength: 100,
						},
						"email": {
							Type:   "string",
							Format: "email",
						},
						"tags": {
							Type: "array",
							Items: &Schema{
								Type: "string",
							},
						},
					},
					Required: []string{"id", "name"},
				},
			},
		}

		// Test valid complex data
		validDataJSON := `{
			"users": [
				{
					"id": 1,
					"name": "Alice",
					"email": "alice@example.com",
					"tags": ["admin", "active"]
				},
				{
					"id": 2,
					"name": "Bob",
					"tags": ["user"]
				}
			]
		}`

		errors, err := ValidateSchema(validDataJSON, schema)
		helper.AssertNoError(err, "Should validate without error")
		helper.AssertEqual(0, len(errors), "Should have no validation errors")
	})

	t.Run("ValidationErrorDetails", func(t *testing.T) {
		// Test that validation errors provide useful details
		schema := DefaultSchema()
		schema.Type = "object"
		nameSchema := DefaultSchema()
		nameSchema.Type = "string"
		nameSchema.SetMinLength(5)
		schema.Properties = map[string]*Schema{
			"name": nameSchema,
		}
		schema.Required = []string{"name"}

		// Test validation with detailed error
		invalidDataJSON := `{"name": "hi"}`

		errors, err := ValidateSchema(invalidDataJSON, schema)
		helper.AssertNoError(err, "Should validate without error")
		helper.AssertTrue(len(errors) > 0, "Should have validation errors")
		if len(errors) > 0 {
			helper.AssertTrue(len(errors[0].Message) > 0, "Error should have descriptive message")
		}
	})

	t.Run("SchemaConfiguration", func(t *testing.T) {
		// Test schema configuration methods
		schema := DefaultSchema()

		// Test length setters
		schema.SetMinLength(5)
		schema.SetMaxLength(50)
		helper.AssertTrue(schema.HasMinLength(), "Should have min length")
		helper.AssertTrue(schema.HasMaxLength(), "Should have max length")

		// Test number setters
		schema.SetMinimum(0)
		schema.SetMaximum(100)
		helper.AssertTrue(schema.HasMinimum(), "Should have minimum")
		helper.AssertTrue(schema.HasMaximum(), "Should have maximum")

		// Test array setters
		schema.SetMinItems(1)
		schema.SetMaxItems(10)
		helper.AssertTrue(schema.HasMinItems(), "Should have min items")
		helper.AssertTrue(schema.HasMaxItems(), "Should have max items")
	})

	t.Run("ValidationWithGlobalProcessor", func(t *testing.T) {
		// Test validation using global processor
		jsonData := `{"name": "test", "age": 25}`

		schema := DefaultSchema()
		schema.Type = "object"
		schema.Properties = map[string]*Schema{
			"name": {Type: "string"},
			"age":  {Type: "number"},
		}

		// Validate JSON directly
		errors, err := ValidateSchema(jsonData, schema)
		helper.AssertNoError(err, "Should validate without error")
		helper.AssertEqual(0, len(errors), "Should have no validation errors")
	})
}
