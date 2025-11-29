package internal

import (
	"testing"
)

func TestPathSegment(t *testing.T) {
	t.Run("PropertySegment", func(t *testing.T) {
		seg := NewPropertySegment("name")

		if seg.Type != PropertySegment {
			t.Error("Type should be PropertySegment")
		}
		if seg.Key != "name" {
			t.Errorf("Expected key 'name', got '%s'", seg.Key)
		}
		if seg.TypeString() != "property" {
			t.Errorf("Expected type string 'property', got '%s'", seg.TypeString())
		}
	})

	t.Run("ArrayIndexSegment", func(t *testing.T) {
		seg := NewArrayIndexSegment(5)

		if seg.Type != ArrayIndexSegment {
			t.Error("Type should be ArrayIndexSegment")
		}
		if seg.Index != 5 {
			t.Errorf("Expected index 5, got %d", seg.Index)
		}
		if seg.TypeString() != "array" {
			t.Errorf("Expected type string 'array', got '%s'", seg.TypeString())
		}
	})

	t.Run("ArraySliceSegment", func(t *testing.T) {
		start := 1
		end := 5
		step := 2

		seg := NewArraySliceSegment(&start, &end, &step)

		if seg.Type != ArraySliceSegment {
			t.Error("Type should be ArraySliceSegment")
		}
		if seg.Start == nil || *seg.Start != 1 {
			t.Error("Start should be 1")
		}
		if seg.End == nil || *seg.End != 5 {
			t.Error("End should be 5")
		}
		if seg.Step == nil || *seg.Step != 2 {
			t.Error("Step should be 2")
		}
		if seg.TypeString() != "slice" {
			t.Errorf("Expected type string 'slice', got '%s'", seg.TypeString())
		}
	})

	t.Run("ExtractSegment", func(t *testing.T) {
		seg := NewExtractSegment("email")

		if seg.Type != ExtractSegment {
			t.Error("Type should be ExtractSegment")
		}
		if seg.Key != "email" {
			t.Errorf("Expected key 'email', got '%s'", seg.Key)
		}
		if seg.Extract != "email" {
			t.Errorf("Expected extract 'email', got '%s'", seg.Extract)
		}
		if seg.IsFlat {
			t.Error("Should not be flat extraction")
		}
	})

	t.Run("FlatExtractSegment", func(t *testing.T) {
		seg := NewExtractSegment("flat:email")

		if seg.Type != ExtractSegment {
			t.Error("Type should be ExtractSegment")
		}
		if seg.Key != "email" {
			t.Errorf("Expected key 'email', got '%s'", seg.Key)
		}
		if !seg.IsFlat {
			t.Error("Should be flat extraction")
		}
	})

	t.Run("LegacyPathSegment", func(t *testing.T) {
		tests := []struct {
			typeStr  string
			value    string
			expected PathSegmentType
		}{
			{"property", "name", PropertySegment},
			{"array", "[0]", ArrayIndexSegment},
		}

		for _, tt := range tests {
			seg := NewLegacyPathSegment(tt.typeStr, tt.value)
			if seg.Type != tt.expected {
				t.Errorf("Expected type %v, got %v", tt.expected, seg.Type)
			}
			if seg.Value != tt.value {
				t.Errorf("Expected value '%s', got '%s'", tt.value, seg.Value)
			}
		}
	})
}

func TestPathSegmentType(t *testing.T) {
	tests := []struct {
		segmentType PathSegmentType
		expected    string
	}{
		{PropertySegment, "property"},
		{ArrayIndexSegment, "array"},
		{ArraySliceSegment, "slice"},
		{WildcardSegment, "wildcard"},
		{RecursiveSegment, "recursive"},
		{FilterSegment, "filter"},
		{ExtractSegment, "extract"},
	}

	for _, tt := range tests {
		result := tt.segmentType.String()
		if result != tt.expected {
			t.Errorf("Expected '%s', got '%s'", tt.expected, result)
		}
	}
}

func TestOperationContext(t *testing.T) {
	t.Run("Creation", func(t *testing.T) {
		ctx := &OperationContext{
			ExtractionPath: "users[*].email",
			OperationType:  "get",
			TargetArrays: []ArrayLocation{
				{
					ContainerPath:  "users",
					ArrayFieldName: "email",
				},
			},
		}

		if ctx.ExtractionPath != "users[*].email" {
			t.Error("Extraction path not set correctly")
		}
		if ctx.OperationType != "get" {
			t.Error("Operation type not set correctly")
		}
		if len(ctx.TargetArrays) != 1 {
			t.Error("Target arrays not set correctly")
		}
	})
}

func TestMappingInfo(t *testing.T) {
	t.Run("Creation", func(t *testing.T) {
		info := &MappingInfo{
			ArrayFieldName: "items",
			TargetIndices:  []int{0, 1, 2},
		}

		if info.ArrayFieldName != "items" {
			t.Error("Array field name not set correctly")
		}
		if len(info.TargetIndices) != 3 {
			t.Error("Target indices not set correctly")
		}
	})
}

func TestArrayLocation(t *testing.T) {
	t.Run("Creation", func(t *testing.T) {
		loc := ArrayLocation{
			ContainerPath:  "data.users",
			ArrayFieldName: "emails",
		}

		if loc.ContainerPath != "data.users" {
			t.Error("Container path not set correctly")
		}
		if loc.ArrayFieldName != "emails" {
			t.Error("Array field name not set correctly")
		}
	})
}
