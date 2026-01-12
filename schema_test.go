package json

import (
	"testing"
)

// TestSchema tests the Schema type
func TestSchema(t *testing.T) {
	schema := &Schema{}

	// Test DefaultSchema initialization
	defaultSchema := DefaultSchema()
	if defaultSchema == nil {
		t.Errorf("DefaultSchema() should not return nil")
	}

	// Test SetMinLength
	schema.SetMinLength(5)
	if !schema.HasMinLength() {
		t.Errorf("HasMinLength() should return true after SetMinLength")
	}

	// Test SetMaxLength
	schema.SetMaxLength(100)
	if !schema.HasMaxLength() {
		t.Errorf("HasMaxLength() should return true after SetMaxLength")
	}

	// Test SetMinimum
	schema.SetMinimum(0)
	if !schema.HasMinimum() {
		t.Errorf("HasMinimum() should return true after SetMinimum")
	}

	// Test SetMaximum
	schema.SetMaximum(1000)
	if !schema.HasMaximum() {
		t.Errorf("HasMaximum() should return true after SetMaximum")
	}

	// Test SetMinItems
	schema.SetMinItems(1)
	if !schema.HasMinItems() {
		t.Errorf("HasMinItems() should return true after SetMinItems")
	}

	// Test SetMaxItems
	schema.SetMaxItems(10)
	if !schema.HasMaxItems() {
		t.Errorf("HasMaxItems() should return true after SetMaxItems")
	}

	// Test SetExclusiveMinimum (should not affect other fields)
	schema.SetExclusiveMinimum(true)
	if !schema.ExclusiveMinimum {
		t.Errorf("ExclusiveMinimum should be true")
	}
	if schema.MinLength != 5 {
		t.Errorf("MinLength should still be 5 after SetExclusiveMinimum, got %d", schema.MinLength)
	}

	// Test SetExclusiveMaximum (should not affect other fields)
	schema.SetExclusiveMaximum(true)
	if !schema.ExclusiveMaximum {
		t.Errorf("ExclusiveMaximum should be true")
	}
	if schema.MaxLength != 100 {
		t.Errorf("MaxLength should still be 100 after SetExclusiveMaximum, got %d", schema.MaxLength)
	}
}

// TestSchema_DefaultSchema tests the DefaultSchema function
func TestSchema_DefaultSchema(t *testing.T) {
	schema := DefaultSchema()

	if schema == nil {
		t.Fatal("DefaultSchema() returned nil")
	}

	// Verify defaults
	if schema.HasMinLength() {
		t.Errorf("Default schema should not have MinLength set")
	}
	if schema.HasMaxLength() {
		t.Errorf("Default schema should not have MaxLength set")
	}
	if schema.HasMinimum() {
		t.Errorf("Default schema should not have Minimum set")
	}
	if schema.HasMaximum() {
		t.Errorf("Default schema should not have Maximum set")
	}
	if schema.HasMinItems() {
		t.Errorf("Default schema should not have MinItems set")
	}
	if schema.HasMaxItems() {
		t.Errorf("Default schema should not have MaxItems set")
	}
}

// TestSchema_StringConstraints tests string constraint methods
func TestSchema_StringConstraints(t *testing.T) {
	schema := &Schema{}

	// Test SetMinLength and HasMinLength
	schema.SetMinLength(10)
	if !schema.HasMinLength() {
		t.Errorf("HasMinLength() should return true")
	}
	if schema.MinLength != 10 {
		t.Errorf("MinLength = %d, want 10", schema.MinLength)
	}

	// Test SetMaxLength and HasMaxLength
	schema.SetMaxLength(100)
	if !schema.HasMaxLength() {
		t.Errorf("HasMaxLength() should return true")
	}
	if schema.MaxLength != 100 {
		t.Errorf("MaxLength = %d, want 100", schema.MaxLength)
	}

	// Test multiple sets
	schema.SetMinLength(5)
	schema.SetMaxLength(50)
	if schema.MinLength != 5 || schema.MaxLength != 50 {
		t.Errorf("MinLength/MaxLength not updated correctly")
	}
}

// TestSchema_NumericConstraints tests numeric constraint methods
func TestSchema_NumericConstraints(t *testing.T) {
	schema := &Schema{}

	// Test SetMinimum and HasMinimum
	schema.SetMinimum(-100)
	if !schema.HasMinimum() {
		t.Errorf("HasMinimum() should return true")
	}
	if schema.Minimum != -100 {
		t.Errorf("Minimum = %f, want -100", schema.Minimum)
	}

	// Test SetMaximum and HasMaximum
	schema.SetMaximum(1000)
	if !schema.HasMaximum() {
		t.Errorf("HasMaximum() should return true")
	}
	if schema.Maximum != 1000 {
		t.Errorf("Maximum = %f, want 1000", schema.Maximum)
	}

	// Test zero values
	schema2 := &Schema{}
	schema2.SetMinimum(0)
	if !schema2.HasMinimum() {
		t.Errorf("HasMinimum() should return true for 0")
	}

	schema2.SetMaximum(0)
	if !schema2.HasMaximum() {
		t.Errorf("HasMaximum() should return true for 0")
	}
}

// TestSchema_ArrayConstraints tests array constraint methods
func TestSchema_ArrayConstraints(t *testing.T) {
	schema := &Schema{}

	// Test SetMinItems and HasMinItems
	schema.SetMinItems(1)
	if !schema.HasMinItems() {
		t.Errorf("HasMinItems() should return true")
	}
	if schema.MinItems != 1 {
		t.Errorf("MinItems = %d, want 1", schema.MinItems)
	}

	// Test SetMaxItems and HasMaxItems
	schema.SetMaxItems(100)
	if !schema.HasMaxItems() {
		t.Errorf("HasMaxItems() should return true")
	}
	if schema.MaxItems != 100 {
		t.Errorf("MaxItems = %d, want 100", schema.MaxItems)
	}

	// Test zero values
	schema2 := &Schema{}
	schema2.SetMinItems(0)
	if !schema2.HasMinItems() {
		t.Errorf("HasMinItems() should return true for 0")
	}

	schema2.SetMaxItems(0)
	if !schema2.HasMaxItems() {
		t.Errorf("HasMaxItems() should return true for 0")
	}
}

// TestSchema_ExclusiveConstraints tests exclusive constraint methods
func TestSchema_ExclusiveConstraints(t *testing.T) {
	schema := &Schema{}

	// Test SetExclusiveMinimum
	schema.SetExclusiveMinimum(true)
	if !schema.ExclusiveMinimum {
		t.Errorf("ExclusiveMinimum should be true")
	}

	schema.SetExclusiveMinimum(false)
	if schema.ExclusiveMinimum {
		t.Errorf("ExclusiveMinimum should be false")
	}

	// Test SetExclusiveMaximum
	schema.SetExclusiveMaximum(true)
	if !schema.ExclusiveMaximum {
		t.Errorf("ExclusiveMaximum should be true")
	}

	schema.SetExclusiveMaximum(false)
	if schema.ExclusiveMaximum {
		t.Errorf("ExclusiveMaximum should be false")
	}

	// Test with other constraints set
	schema.SetMinimum(10)
	schema.SetExclusiveMinimum(true)
	if schema.Minimum != 10 {
		t.Errorf("Minimum should still be 10")
	}

	schema.SetMaximum(100)
	schema.SetExclusiveMaximum(true)
	if schema.Maximum != 100 {
		t.Errorf("Maximum should still be 100")
	}
}

// TestSchema_AllConstraints tests setting all constraints together
func TestSchema_AllConstraints(t *testing.T) {
	schema := &Schema{}

	schema.SetMinLength(5)
	schema.SetMaxLength(50)
	schema.SetMinimum(0)
	schema.SetMaximum(100)
	schema.SetMinItems(1)
	schema.SetMaxItems(10)
	schema.SetExclusiveMinimum(false)
	schema.SetExclusiveMaximum(false)

	if !schema.HasMinLength() || schema.MinLength != 5 {
		t.Errorf("MinLength not set correctly")
	}
	if !schema.HasMaxLength() || schema.MaxLength != 50 {
		t.Errorf("MaxLength not set correctly")
	}
	if !schema.HasMinimum() || schema.Minimum != 0 {
		t.Errorf("Minimum not set correctly")
	}
	if !schema.HasMaximum() || schema.Maximum != 100 {
		t.Errorf("Maximum not set correctly")
	}
	if !schema.HasMinItems() || schema.MinItems != 1 {
		t.Errorf("MinItems not set correctly")
	}
	if !schema.HasMaxItems() || schema.MaxItems != 10 {
		t.Errorf("MaxItems not set correctly")
	}
	if schema.ExclusiveMinimum {
		t.Errorf("ExclusiveMinimum should be false")
	}
	if schema.ExclusiveMaximum {
		t.Errorf("ExclusiveMaximum should be false")
	}
}
