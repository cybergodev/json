package json

// Set operation methods have been refactored into separate files for better organization:
//
//   - ops_set_core.go    : Core set methods (setValueAtPath, setValueWithSegments, etc.)
//   - ops_set_array.go   : Array extension and index/slice operations
//   - ops_set_navigate.go: Navigation methods (navigateToSegment, navigateToProperty, etc.)
//   - ops_set_extract.go : Extraction-related set operations
//   - ops_set_jsonpointer.go: JSON Pointer format support (existing file)
//
// This file is kept for backward compatibility and as a documentation reference.
// All methods remain as private Processor methods in package json.
