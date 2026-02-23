package json

// File operation methods have been refactored into separate files for better organization:
//
//   - file_read.go    : LoadFromFile, LoadFromFileAsData, LoadFromReader, etc.
//   - file_write.go   : SaveToFile, SaveToWriter, MarshalToFile, UnmarshalFromFile
//   - file_validate.go: validateFilePath, containsPathTraversal, validateUnixPath, validateWindowsPath
//
// This file is kept for backward compatibility and as a documentation reference.
// All methods remain as private Processor methods in package json.
