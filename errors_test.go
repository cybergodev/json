package json

import (
	"errors"
	"testing"
)

// TestJsonsErrorIs verifies the Is method including nil receiver.
func TestJsonsErrorIs(t *testing.T) {
	tests := []struct {
		name   string
		receiver *JsonsError
		target error
		want    bool
	}{
		{
			name:     "nil receiver with nil target",
			receiver: nil,
			target:   nil,
			want:     true,
		},
		{
			name:     "nil receiver with non-nil target",
			receiver: nil,
			target:   ErrInvalidJSON,
			want:     false,
		},
		{
			name:     "non-nil receiver with nil target",
			receiver: &JsonsError{Op: "get", Message: "test"},
			target:   nil,
			want:     false,
		},
		{
			name:     "matching Op, Path, Err",
			receiver: &JsonsError{Op: "get", Path: "data", Err: ErrPathNotFound},
			target:   &JsonsError{Op: "get", Path: "data", Err: ErrPathNotFound},
			want:     true,
		},
		{
			name:     "mismatched Op",
			receiver: &JsonsError{Op: "get", Path: "data", Err: ErrPathNotFound},
			target:   &JsonsError{Op: "set", Path: "data", Err: ErrPathNotFound},
			want:     false,
		},
		{
			name:     "mismatched Path",
			receiver: &JsonsError{Op: "get", Path: "data", Err: ErrPathNotFound},
			target:   &JsonsError{Op: "get", Path: "other", Err: ErrPathNotFound},
			want:     false,
		},
		{
			name:     "mismatched Err",
			receiver: &JsonsError{Op: "get", Path: "data", Err: ErrPathNotFound},
			target:   &JsonsError{Op: "get", Path: "data", Err: ErrInvalidJSON},
			want:     false,
		},
		{
			name:     "nil Err fields match",
			receiver: &JsonsError{Op: "get", Path: "data"},
			target:   &JsonsError{Op: "get", Path: "data"},
			want:     true,
		},
		{
			name:     "one nil Err other non-nil Err",
			receiver: &JsonsError{Op: "get", Path: "data"},
			target:   &JsonsError{Op: "get", Path: "data", Err: ErrPathNotFound},
			want:     false,
		},
		{
			name:     "underlying sentinel matches via errors.Is",
			receiver: &JsonsError{Op: "get", Message: "not found", Err: ErrPathNotFound},
			target:   ErrPathNotFound,
			want:     true,
		},
		{
			name:     "underlying sentinel does not match",
			receiver: &JsonsError{Op: "get", Message: "not found", Err: ErrPathNotFound},
			target:   ErrInvalidJSON,
			want:     false,
		},
		{
			name:     "no underlying error, target is sentinel",
			receiver: &JsonsError{Op: "get", Message: "test"},
			target:   ErrInvalidJSON,
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.receiver.Is(tt.target)
			if got != tt.want {
				t.Errorf("JsonsError.Is() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestSafeError verifies SafeError strips internal context from errors.
func TestSafeError(t *testing.T) {
	tests := []struct {
		name  string
		input error
		want  string
	}{
		{"nil returns empty", nil, ""},
		{"JsonsError returns sentinel message", &JsonsError{Op: "get", Path: "users.admin.password", Message: "not found", Err: ErrPathNotFound}, "path not found"},
		{"plain error returns full message", errors.New("something went wrong"), "something went wrong"},
		{"security error strips context", newSecurityError("parse", "dangerous input"), "security violation detected"},
		{"size limit error strips context", newSizeLimitError("load", 1<<30, 1<<20), "size limit exceeded"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SafeError(tt.input)
			if got != tt.want {
				t.Errorf("SafeError() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestRedactedPath verifies path redaction for safe logging.
func TestRedactedPath(t *testing.T) {
	tests := []struct {
		name string
		path string
		want string
	}{
		{"empty returns empty", "", ""},
		{"short path returns masked", "users.name", "***"},
		{"exactly 32 returns masked", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "***"},
		{"long path returns truncated", "this.is.a.very.long.path.that.exceeds.thirty.two.characters", "this.is....aracters"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := RedactedPath(tt.path)
			if got != tt.want {
				t.Errorf("RedactedPath(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}
