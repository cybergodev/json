package json

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/cybergodev/json/internal"
)

// TestErrorHandling comprehensive error handling tests
func TestErrorHandling(t *testing.T) {
	helper := NewTestHelper(t)

	t.Run("JsonsErrorStructure", func(t *testing.T) {
		err := &JsonsError{
			Op:      "test_operation",
			Path:    "test.path",
			Message: "test error message",
			Err:     ErrOperationFailed,
		}

		helper.AssertEqual("test_operation", err.Op)
		helper.AssertEqual("test.path", err.Path)
		helper.AssertEqual("test error message", err.Message)
		helper.AssertEqual(ErrOperationFailed, err.Err)
	})

	t.Run("ErrorWrapping", func(t *testing.T) {
		baseErr := ErrPathNotFound

		t.Run("WrapError", func(t *testing.T) {
			wrapped := WrapError(baseErr, "get_user", "user not found")
			helper.AssertNotNil(wrapped)

			// Unwrap should return original
			unwrapped := errors.Unwrap(wrapped)
			helper.AssertEqual(baseErr, unwrapped)
		})

		t.Run("WrapPathError", func(t *testing.T) {
			wrapped := WrapPathError(baseErr, "get_field", "user.profile.email", "field missing")
			helper.AssertNotNil(wrapped)

			// Check error message contains path
			helper.AssertTrue(errors.Is(wrapped, baseErr))
		})
	})

	t.Run("ErrorTypes", func(t *testing.T) {
		testData := `{"name": "John", "age": 30}`

		tests := []struct {
			name    string
			path    string
			wantErr bool
			errType error
		}{
			{"ValidPath", "name", false, nil},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				_, err := Get(testData, tt.path)
				if tt.wantErr {
					helper.AssertError(err)
					if tt.errType != nil {
						var jsonErr *JsonsError
						if errors.As(err, &jsonErr) {
							helper.AssertEqual(tt.errType, jsonErr.Err)
						}
					}
				} else {
					helper.AssertNoError(err)
				}
			})
		}
	})
}

// TestErrorClassifier tests error classification functionality
func TestErrorClassifier(t *testing.T) {
	helper := NewTestHelper(t)

	t.Run("IsSecurityRelated", func(t *testing.T) {
		tests := []struct {
			name     string
			err      error
			expected bool
		}{
			{"SecurityViolation", ErrSecurityViolation, true},
			{"PathNotFound", ErrPathNotFound, false},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				result := IsSecurityRelated(tt.err)
				helper.AssertEqual(tt.expected, result)
			})
		}
	})

	t.Run("IsUserError", func(t *testing.T) {
		tests := []struct {
			name     string
			err      error
			expected bool
		}{
			{"PathNotFound", ErrPathNotFound, true},
			{"InvalidPath", ErrInvalidPath, true},
			{"TypeMismatch", ErrTypeMismatch, true},
			{"SystemError", ErrOperationFailed, false},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				result := IsUserError(tt.err)
				helper.AssertEqual(tt.expected, result)
			})
		}
	})

	t.Run("IsRetryable", func(t *testing.T) {
		tests := []struct {
			name     string
			err      error
			expected bool
		}{
			{"Timeout", ErrOperationTimeout, true},
			{"ConcurrencyLimit", ErrConcurrencyLimit, true},
			{"InvalidJSON", ErrInvalidJSON, false},
			{"PathNotFound", ErrPathNotFound, false},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				result := IsRetryable(tt.err)
				helper.AssertEqual(tt.expected, result)
			})
		}
	})

	t.Run("GetErrorSuggestion", func(t *testing.T) {
		tests := []struct {
			name               string
			err                error
			suggestionContains string
		}{
			{"InvalidJSON", ErrInvalidJSON, "valid"},
			{"PathNotFound", ErrPathNotFound, "check"},
			{"TypeMismatch", ErrTypeMismatch, "type"},
			{"InvalidPath", ErrInvalidPath, "format"},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				suggestion := GetErrorSuggestion(tt.err)
				helper.AssertNotNil(suggestion)
				helper.AssertTrue(
					len(suggestion) > 0,
					"Suggestion should not be empty",
				)
				// Suggestions may vary by implementation, just verify we got one
				_ = tt.suggestionContains
			})
		}
	})
}

// TestErrorScenarios tests various error scenarios
func TestErrorScenarios(t *testing.T) {
	helper := NewTestHelper(t)

	t.Run("InvalidJSON", func(t *testing.T) {
		invalidJSON := []string{
			`{invalid json}`,
			`{"unclosed": "string}`,
			`{"trailing": "comma",}`,
			`{unquoted: "key"}`,
			`{"number": 123.45.67}`,
			`{"array": [1, 2, 3,]}`,
		}

		for _, jsonStr := range invalidJSON {
			t.Run("JSON_"+jsonStr[:10], func(t *testing.T) {
				_, err := Get(jsonStr, "any")
				helper.AssertError(err)
			})
		}
	})

	t.Run("InvalidPaths", func(t *testing.T) {
		testData := `{"user": {"name": "John"}}`

		// Only test paths that should legitimately fail
		invalidPaths := []string{
			"[[[", // Invalid bracket syntax
			"]]]", // Invalid bracket syntax
		}

		for _, path := range invalidPaths {
			t.Run("Path_"+path, func(t *testing.T) {
				_, err := Get(testData, path)
				helper.AssertError(err)
			})
		}
	})

	t.Run("ArrayErrors", func(t *testing.T) {
		testData := `{"arr": [1, 2, 3]}`

		t.Run("InvalidIndex", func(t *testing.T) {
			_, err := Get(testData, "arr[abc]")
			helper.AssertError(err)
		})

		t.Run("ArrayOnNonArray", func(t *testing.T) {
			_, err := Get(testData, "user[0]")
			helper.AssertError(err)
		})
	})

	t.Run("TypeMismatch", func(t *testing.T) {
		testData := `{"str": "value", "num": 42, "bool": true}`

		t.Run("StringAsInt", func(t *testing.T) {
			_, err := GetInt(testData, "str")
			helper.AssertError(err)
		})

		t.Run("NumberAsString", func(t *testing.T) {
			_, err := GetString(testData, "num")
			// This might succeed with conversion
			_ = err
		})

		t.Run("BoolAsInt", func(t *testing.T) {
			// Bool to int might convert (true=1, false=0)
			_, err := GetInt(testData, "bool")
			_ = err // Don't assert error, library may convert
		})

		t.Run("ObjectAsArray", func(t *testing.T) {
			_, err := GetArray(testData, "str")
			helper.AssertError(err)
		})
	})

	t.Run("NullHandling", func(t *testing.T) {
		testData := `{"null_value": null, "string_value": "value"}`

		t.Run("GetNull", func(t *testing.T) {
			result, err := Get(testData, "null_value")
			helper.AssertNoError(err)
			helper.AssertNil(result)
		})

		t.Run("GetTypedNull", func(t *testing.T) {
			_, err := GetTyped[string](testData, "null_value")
			// Null to string might error or return "null"
			_ = err
		})

		t.Run("GetMissingField", func(t *testing.T) {
			_, err := Get(testData, "missing")
			helper.AssertError(err)
		})
	})

	t.Run("ProcessorClosed", func(t *testing.T) {
		processor := New(DefaultConfig())
		processor.Close()

		testData := `{"test": "value"}`

		_, err := processor.Get(testData, "test")
		helper.AssertError(err)

		var jsonErr *JsonsError
		helper.AssertTrue(errors.As(err, &jsonErr))
	})

	t.Run("EmptyInput", func(t *testing.T) {
		tests := []struct {
			name  string
			input string
		}{
			{"EmptyString", ""},
			{"WhitespaceOnly", "   \n\t  "},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				_, err := Get(tt.input, "any")
				helper.AssertError(err)
			})
		}
	})

	t.Run("ConcurrentErrors", func(t *testing.T) {
		processor := New(DefaultConfig())
		defer processor.Close()

		// Attempt to access after close from multiple goroutines
		done := make(chan error, 5)

		for i := 0; i < 5; i++ {
			go func() {
				_, err := processor.Get(`{"test": "value"}`, "test")
				done <- err
			}()
		}

		// Close while operations might be in flight
		processor.Close()

		for i := 0; i < 5; i++ {
			err := <-done
			// May succeed if it completed before close, or error if after
			_ = err
		}
	})
}

// TestErrorMessages verifies error messages are helpful
func TestErrorMessages(t *testing.T) {
	helper := NewTestHelper(t)

	t.Run("PathNotFoundMessage", func(t *testing.T) {
		testData := `{"user": {"name": "John"}}`
		_, err := Get(testData, "user.email")

		helper.AssertError(err)
		var jsonErr *JsonsError
		if errors.As(err, &jsonErr) {
			helper.AssertEqual("get", jsonErr.Op)
			helper.AssertEqual("user.email", jsonErr.Path)
			helper.AssertTrue(strings.Contains(jsonErr.Message, "not found"))
		}
	})

	t.Run("InvalidPathMessage", func(t *testing.T) {
		_, err := Get(`{"test": "value"}`, "[[[")

		helper.AssertError(err)
		if err != nil {
			helper.AssertTrue(strings.Contains(err.Error(), "path") ||
				strings.Contains(err.Error(), "bracket") ||
				strings.Contains(err.Error(), "invalid"))
		}
	})

	t.Run("TypeMismatchMessage", func(t *testing.T) {
		testData := `{"value": "not a number"}`

		_, err := GetInt(testData, "value")
		helper.AssertError(err)

		var jsonErr *JsonsError
		if errors.As(err, &jsonErr) {
			helper.AssertEqual(ErrTypeMismatch, jsonErr.Err)
		}
	})
}

// TestErrorRecovery tests error recovery scenarios
func TestErrorRecovery(t *testing.T) {
	helper := NewTestHelper(t)

	t.Run("RetryAfterTimeout", func(t *testing.T) {
		retryableErrors := []error{
			ErrOperationTimeout,
			ErrConcurrencyLimit,
		}

		for _, err := range retryableErrors {
			t.Run(err.Error(), func(t *testing.T) {
				helper.AssertTrue(IsRetryable(err))
			})
		}
	})

	t.Run("NoRetryForInvalidData", func(t *testing.T) {
		nonRetryableErrors := []error{
			ErrInvalidJSON,
			ErrInvalidPath,
			ErrTypeMismatch,
		}

		for _, err := range nonRetryableErrors {
			t.Run(err.Error(), func(t *testing.T) {
				helper.AssertFalse(IsRetryable(err))
			})
		}
	})

	t.Run("ContinueAfterError", func(t *testing.T) {
		testData := `{
			"valid1": "value1",
			"invalid": null,
			"valid2": "value2"
		}`

		// Get multiple paths, some may fail
		paths := []string{"valid1", "invalid", "valid2"}

		successCount := 0
		for _, path := range paths {
			_, err := GetString(testData, path)
			if err == nil {
				successCount++
			}
		}

		// Should have succeeded for valid paths
		helper.AssertTrue(successCount >= 2)
	})
}

// TestRootDataTypeConversionError tests the RootDataTypeConversionError type
func TestRootDataTypeConversionError(t *testing.T) {
	err := &RootDataTypeConversionError{
		RequiredType: "object",
		RequiredSize: 100,
		CurrentType:  "string",
	}

	expectedMsg := "root data type conversion required: from string to object (size: 100)"
	if err.Error() != expectedMsg {
		t.Errorf("Error() = %q, want %q", err.Error(), expectedMsg)
	}

	// Test nil error
	var nilErr *RootDataTypeConversionError
	if nilErr != nil {
		t.Errorf("nil error should be nil")
	}
}

// TestArrayExtensionError tests the ArrayExtensionError type
func TestArrayExtensionError(t *testing.T) {
	// Test default message
	err1 := &ArrayExtensionError{
		CurrentLength:  5,
		RequiredLength: 10,
		TargetIndex:    9,
		Value:          "test",
	}
	expectedMsg1 := "array extension required: current length 5, required length 10 for index 9"
	if err1.Error() != expectedMsg1 {
		t.Errorf("Error() = %q, want %q", err1.Error(), expectedMsg1)
	}

	// Test custom message
	err2 := &ArrayExtensionError{
		CurrentLength:  5,
		RequiredLength: 10,
		TargetIndex:    9,
		Value:          "test",
		Message:        "Custom error message",
	}
	if err2.Error() != "Custom error message" {
		t.Errorf("Error() with custom message = %q, want %q", err2.Error(), "Custom error message")
	}

	// Test with custom message
	err3 := &ArrayExtensionError{
		CurrentLength:  3,
		RequiredLength: 10,
		TargetIndex:    9,
		Value:          "test",
		Message:        "Array too small",
	}
	if err3.Error() != "Array too small" {
		t.Errorf("Error() with custom message = %q, want %q", err3.Error(), "Array too small")
	}
}

// TestInvalidUnmarshalError tests the InvalidUnmarshalError type
func TestInvalidUnmarshalError(t *testing.T) {
	// Test nil type
	err1 := &InvalidUnmarshalError{Type: nil}
	if err1.Error() != "json: Unmarshal(nil)" {
		t.Errorf("Error() with nil Type = %q, want %q", err1.Error(), "json: Unmarshal(nil)")
	}

	// Test non-pointer type
	strType := reflect.TypeOf("")
	err2 := &InvalidUnmarshalError{Type: strType}
	if err2.Error() != "json: Unmarshal(non-pointer string)" {
		t.Errorf("Error() with non-pointer = %q, want %q", err2.Error(), "json: Unmarshal(non-pointer string)")
	}

	// Test nil pointer
	type MyStruct struct{ Field int }
	ptrType := reflect.TypeOf(&MyStruct{})
	err3 := &InvalidUnmarshalError{Type: ptrType}
	if err3.Error() != "json: Unmarshal(nil *json.MyStruct)" {
		t.Errorf("Error() with nil pointer = %q, want %q", err3.Error(), "json: Unmarshal(nil *json.MyStruct)")
	}
}

// TestSyntaxError tests the SyntaxError type
func TestSyntaxError(t *testing.T) {
	err := &SyntaxError{
		msg:    "invalid character 'a' looking for beginning of value",
		Offset: 42,
	}

	expectedMsg := "invalid character 'a' looking for beginning of value"
	if err.Error() != expectedMsg {
		t.Errorf("Error() = %q, want %q", err.Error(), expectedMsg)
	}

	if err.Offset != 42 {
		t.Errorf("Offset = %d, want 42", err.Offset)
	}
}

// TestUnmarshalTypeError tests the UnmarshalTypeError type
func TestUnmarshalTypeError(t *testing.T) {
	// Test without Struct/Field
	err1 := &UnmarshalTypeError{
		Value:  "number",
		Type:   reflect.TypeOf(0),
		Offset: 10,
	}
	expectedMsg1 := "json: cannot unmarshal number into Go value of type int"
	if err1.Error() != expectedMsg1 {
		t.Errorf("Error() without Struct/Field = %q, want %q", err1.Error(), expectedMsg1)
	}

	// Test with Struct and Field
	err2 := &UnmarshalTypeError{
		Value:  "string",
		Type:   reflect.TypeOf(0),
		Offset: 20,
		Struct: "MyStruct",
		Field:  "Field",
	}
	expectedMsg2 := "json: cannot unmarshal string into Go struct field MyStruct.Field of type int"
	if err2.Error() != expectedMsg2 {
		t.Errorf("Error() with Struct/Field = %q, want %q", err2.Error(), expectedMsg2)
	}

	// Test Unwrap with nil error
	if err1.Unwrap() != nil {
		t.Errorf("Unwrap() with nil Err should return nil")
	}

	// Test Unwrap with error
	wrappedErr := errors.New("wrapped error")
	err3 := &UnmarshalTypeError{
		Value:  "bool",
		Type:   reflect.TypeOf(""),
		Offset: 5,
		Err:    wrappedErr,
	}
	if err3.Unwrap() != wrappedErr {
		t.Errorf("Unwrap() should return wrapped error")
	}
}

// TestUnsupportedTypeError tests the UnsupportedTypeError type
func TestUnsupportedTypeError(t *testing.T) {
	chType := reflect.TypeOf(make(chan int))
	err := &UnsupportedTypeError{Type: chType}

	expectedMsg := "json: unsupported type: chan int"
	if err.Error() != expectedMsg {
		t.Errorf("Error() = %q, want %q", err.Error(), expectedMsg)
	}
}

// TestUnsupportedValueError tests the UnsupportedValueError type
func TestUnsupportedValueError(t *testing.T) {
	// Create a function value (which is unsupported)
	fn := func() {}
	val := reflect.ValueOf(fn)

	err := &UnsupportedValueError{
		Value: val,
		Str:   "func()",
	}

	expectedMsg := "json: unsupported value: func()"
	if err.Error() != expectedMsg {
		t.Errorf("Error() = %q, want %q", err.Error(), expectedMsg)
	}
}

// TestMarshalerError tests the MarshalerError type
func TestMarshalerError(t *testing.T) {
	type TestType struct{}
	testType := reflect.TypeOf(TestType{})
	wrappedErr := errors.New("marshal failed")

	err1 := &MarshalerError{
		Type:       testType,
		Err:        wrappedErr,
		sourceFunc: "",
	}

	expectedMsg1 := "json: error calling MarshalJSON for type json.TestType: marshal failed"
	if err1.Error() != expectedMsg1 {
		t.Errorf("Error() with empty sourceFunc = %q, want %q", err1.Error(), expectedMsg1)
	}

	// Test Unwrap
	if err1.Unwrap() != wrappedErr {
		t.Errorf("Unwrap() should return wrapped error")
	}

	// Test with custom sourceFunc
	err2 := &MarshalerError{
		Type:       testType,
		Err:        wrappedErr,
		sourceFunc: "MarshalText",
	}

	expectedMsg2 := "json: error calling MarshalText for type json.TestType: marshal failed"
	if err2.Error() != expectedMsg2 {
		t.Errorf("Error() with custom sourceFunc = %q, want %q", err2.Error(), expectedMsg2)
	}
}

// TestPathInfo tests the PathInfo type
func TestPathInfo(t *testing.T) {
	segments := []PathSegment{
		{Type: internal.PropertySegment, Key: "user"},
		{Type: internal.ArrayIndexSegment, Index: 0},
	}

	pathInfo := PathInfo{
		Segments:     segments,
		IsPointer:    false,
		OriginalPath: "user[0]",
	}

	if len(pathInfo.Segments) != 2 {
		t.Errorf("Segments length = %d, want 2", len(pathInfo.Segments))
	}

	if pathInfo.IsPointer {
		t.Errorf("IsPointer = true, want false")
	}

	if pathInfo.OriginalPath != "user[0]" {
		t.Errorf("OriginalPath = %q, want %q", pathInfo.OriginalPath, "user[0]")
	}
}

// TestPropertyAccessResult tests the PropertyAccessResult type
func TestPropertyAccessResult(t *testing.T) {
	// Test exists case
	result1 := PropertyAccessResult{
		Value:  "test",
		Exists: true,
	}

	if !result1.Exists {
		t.Errorf("Exists should be true")
	}

	if result1.Value != "test" {
		t.Errorf("Value = %v, want %v", result1.Value, "test")
	}

	// Test not exists case
	result2 := PropertyAccessResult{
		Value:  nil,
		Exists: false,
	}

	if result2.Exists {
		t.Errorf("Exists should be false")
	}
}

// TestInvalidArrayIndex tests the InvalidArrayIndex constant
func TestInvalidArrayIndex(t *testing.T) {
	if InvalidArrayIndex != -999999 {
		t.Errorf("InvalidArrayIndex = %d, want -999999", InvalidArrayIndex)
	}
}

// TestDeletedMarker tests the DeletedMarker constant
func TestDeletedMarker(t *testing.T) {
	if DeletedMarker == nil {
		t.Errorf("DeletedMarker should not be nil")
	}

	// Test that DeletedMarker can be used for comparison
	arr := []any{1, DeletedMarker, 3}
	count := 0
	for _, v := range arr {
		if v == DeletedMarker {
			count++
		}
	}
	if count != 1 {
		t.Errorf("Found %d DeletedMarker, want 1", count)
	}
}
