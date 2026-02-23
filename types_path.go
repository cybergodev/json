package json

import (
	"fmt"

	"github.com/cybergodev/json/internal"
)

// PropertyAccessResult represents the result of a property access operation
type PropertyAccessResult struct {
	Value  any
	Exists bool
	Path   string // Path that was accessed
	Error  error
}

// RootDataTypeConversionError signals that root data type conversion is needed
type RootDataTypeConversionError struct {
	RequiredType string
	RequiredSize int
	CurrentType  string
}

func (e *RootDataTypeConversionError) Error() string {
	return fmt.Sprintf("root data type conversion required: from %s to %s (size: %d)",
		e.CurrentType, e.RequiredType, e.RequiredSize)
}

// ArrayExtensionError signals that array extension is needed
type ArrayExtensionError struct {
	CurrentLength  int
	RequiredLength int
	TargetIndex    int
	Value          any
	Message        string
}

func (e *ArrayExtensionError) Error() string {
	if e.Message != "" {
		return e.Message
	}
	return fmt.Sprintf("array extension required: current length %d, required length %d for index %d",
		e.CurrentLength, e.RequiredLength, e.TargetIndex)
}

// PathSegment represents a parsed path segment with its type and value
type PathSegment = internal.PathSegment

// ExtractionGroup represents a group of consecutive extraction segments
type ExtractionGroup = internal.ExtractionGroup

// PathInfo contains parsed path information
type PathInfo struct {
	Segments     []PathSegment `json:"segments"`
	IsPointer    bool          `json:"is_pointer"`
	OriginalPath string        `json:"original_path"`
}
