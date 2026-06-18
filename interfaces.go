// Package json provides extension interfaces for customizing JSON processing behavior.
// These interfaces enable users to inject custom validation, encoding, and middleware logic.
package json

import (
	"fmt"
	"log/slog"
	"reflect"
	"time"

	"github.com/cybergodev/json/internal"
)

// =============================================================================
// 1. Encoder Interfaces
// =============================================================================

// CustomEncoder provides custom JSON encoding capability.
// Implement this interface to replace the default encoder entirely.
//
// Example:
//
//	type UpperCaseEncoder struct{}
//	func (e *UpperCaseEncoder) Encode(value any) (string, error) {
//	    // Custom encoding logic
//	}
//
//	cfg := json.DefaultConfig()
//	cfg.CustomEncoder = &UpperCaseEncoder{}
type CustomEncoder interface {
	// Encode converts a Go value to JSON string.
	// Returns an error if value cannot be encoded.
	Encode(value any) (string, error)
}

// TypeEncoder handles encoding for specific reflect.Types.
// Register via Config.CustomTypeEncoders map.
//
// Example:
//
//	type TimeEncoder struct{}
//	func (e *TimeEncoder) Encode(v reflect.Value) (string, error) {
//	    t := v.Interface().(time.Time)
//	    return `"` + t.Format(time.RFC3339) + `"`, nil
//	}
//
//	cfg := json.DefaultConfig()
//	cfg.CustomTypeEncoders = map[reflect.Type]json.TypeEncoder{
//	    reflect.TypeOf(time.Time{}): &TimeEncoder{},
//	}
type TypeEncoder interface {
	// Encode converts a specific type to its JSON representation.
	// Return the JSON string (including quotes for strings) or an error.
	Encode(v reflect.Value) (string, error)
}

// =============================================================================
// 2. Validator Interfaces
// =============================================================================

// Validator validates JSON input before processing.
// Implement this interface to add custom validation logic.
//
// Example:
//
//	type SizeValidator struct { MaxSize int64 }
//	func (v *SizeValidator) Validate(jsonStr string) error {
//	    if int64(len(jsonStr)) > v.MaxSize {
//	        return fmt.Errorf("JSON exceeds max size: %d", v.MaxSize)
//	    }
//	    return nil
//	}
type Validator interface {
	// Validate checks JSON string for issues.
	// Returns nil if valid, or an error describing the problem.
	Validate(jsonStr string) error
}

// validationChain runs multiple validators in sequence.
// Stops at the first error encountered.
type validationChain []Validator

// Validate executes all validators in order, stopping at first error.
func (vc validationChain) Validate(jsonStr string) error {
	for _, v := range vc {
		if err := v.Validate(jsonStr); err != nil {
			return err
		}
	}
	return nil
}

// =============================================================================
// 3. Security Pattern Registration
// =============================================================================

// PatternLevel represents the severity level of a dangerous pattern.
type PatternLevel int

const (
	// PatternLevelCritical always blocks the operation.
	// Use for patterns that pose immediate security risks (e.g., prototype pollution).
	PatternLevelCritical PatternLevel = iota

	// PatternLevelWarning blocks in strict mode, logs warning in lenient mode.
	// Use for patterns that may indicate malicious intent but have legitimate uses.
	PatternLevelWarning

	// PatternLevelInfo logs but never blocks.
	// Use for audit/tracking purposes without interrupting operations.
	PatternLevelInfo
)

// String returns the string representation of PatternLevel.
func (pl PatternLevel) String() string {
	switch pl {
	case PatternLevelCritical:
		return "critical"
	case PatternLevelWarning:
		return "warning"
	case PatternLevelInfo:
		return "info"
	default:
		return "unknown"
	}
}

// DangerousPattern represents a security risk pattern to detect.
type DangerousPattern struct {
	// Pattern is the substring to detect in input.
	Pattern string

	// Name is a human-readable description of the security risk.
	Name string

	// Level determines how the pattern is handled.
	Level PatternLevel
}

// =============================================================================
// 4. Path Segment Type
// =============================================================================

// PathSegment represents a parsed path segment.
// This is an alias to internal.PathSegment.
type PathSegment = internal.PathSegment

// =============================================================================
// 5. Hook Interface - Unified Interception Mechanism
// =============================================================================

// HookContext provides context for operation hooks.
type HookContext struct {
	// Operation is the type of operation being performed.
	// Values: "get", "set", "delete", "marshal", "unmarshal"
	Operation string

	// JSONStr is the input JSON string (may be empty for marshal).
	//
	// SECURITY WARNING: This field may contain sensitive data (passwords,
	// tokens, API keys, PII). Do NOT log this value. Only inspect specific
	// paths if needed. Use Operation and Path for logging purposes.
	JSONStr string

	// Path is the target path (may be empty for marshal/unmarshal).
	Path string

	// Value is the value for set operations.
	Value any

	// Config is the active configuration.
	Config *Config

	// StartTime is when the operation started (set before After is called).
	StartTime time.Time
}

// Hook intercepts operations before/after execution.
// Implement this interface to add cross-cutting concerns like logging,
// metrics, tracing, or request transformation.
//
// Example:
//
//	type LoggingHook struct{ logger *slog.Logger }
//
//	func (h *LoggingHook) Before(ctx json.HookContext) error {
//	    h.logger.Info("operation starting", "op", ctx.Operation, "path", ctx.Path)
//	    return nil
//	}
//
//	func (h *LoggingHook) After(ctx json.HookContext, result any, err error) (any, error) {
//	    h.logger.Info("operation completed", "op", ctx.Operation, "error", err)
//	    return result, err
//	}
type Hook interface {
	// Before is called before an operation.
	// Return error to abort the operation.
	Before(ctx HookContext) error

	// After is called after an operation completes.
	// Modify result or error as needed.
	After(ctx HookContext, result any, err error) (any, error)
}

// HookFunc is an adapter to use ordinary functions as Hooks.
// Useful for simple hooks that don't need both Before and After.
//
// Example:
//
//	// Only need After
//	p.AddHook(&json.HookFunc{
//	    AfterFn: func(ctx json.HookContext, result any, err error) (any, error) {
//	        log.Printf("%s completed in %v", ctx.Operation, time.Since(ctx.StartTime))
//	        return result, err
//	    },
//	})
type HookFunc struct {
	BeforeFn func(ctx HookContext) error
	AfterFn  func(ctx HookContext, result any, err error) (any, error)
}

// Before calls the BeforeFn if set, otherwise returns nil.
func (h *HookFunc) Before(ctx HookContext) error {
	if h.BeforeFn != nil {
		return h.BeforeFn(ctx)
	}
	return nil
}

// After calls the AfterFn if set, otherwise returns the original result.
func (h *HookFunc) After(ctx HookContext, result any, err error) (any, error) {
	if h.AfterFn != nil {
		return h.AfterFn(ctx, result, err)
	}
	return result, err
}

// =============================================================================
// Convenience Hook Implementations
// =============================================================================

// LoggingHook creates a hook that logs all operations.
// The logger must implement Info(msg string, args ...any).
//
// Example:
//
//	p.AddHook(json.LoggingHook(slog.Default()))
func LoggingHook(logger interface{ Info(msg string, args ...any) }) Hook {
	return &HookFunc{
		BeforeFn: func(ctx HookContext) error {
			logger.Info("operation starting", "op", ctx.Operation, "path", ctx.Path)
			return nil
		},
		AfterFn: func(ctx HookContext, result any, err error) (any, error) {
			logger.Info("operation completed",
				"op", ctx.Operation,
				"path", ctx.Path,
				"duration", time.Since(ctx.StartTime),
				"error", err)
			return result, err
		},
	}
}

// TimingHook creates a hook that records operation duration.
// The recorder must implement Record(op string, duration time.Duration).
//
// Example:
//
//	p.AddHook(json.TimingHook(myMetricsRecorder))
func TimingHook(recorder interface {
	Record(op string, duration time.Duration)
}) Hook {
	return &HookFunc{
		AfterFn: func(ctx HookContext, result any, err error) (any, error) {
			recorder.Record(ctx.Operation, time.Since(ctx.StartTime))
			return result, err
		},
	}
}

// ValidationHook creates a hook that validates input before operations.
// Return error from the validator to abort the operation.
//
// Example:
//
//	p.AddHook(json.ValidationHook(func(jsonStr, path string) error {
//	    if len(jsonStr) > 1_000_000 {
//	        return errors.New("JSON too large")
//	    }
//	    return nil
//	}))
func ValidationHook(validator func(jsonStr, path string) error) Hook {
	return &HookFunc{
		BeforeFn: func(ctx HookContext) error {
			return validator(ctx.JSONStr, ctx.Path)
		},
	}
}

// ErrorHook creates a hook that intercepts errors.
// The handler can transform errors or log them.
//
// Example:
//
//	p.AddHook(json.ErrorHook(func(ctx json.HookContext, err error) error {
//	    sentry.CaptureException(err)
//	    return err // return original or transformed error
//	}))
func ErrorHook(handler func(ctx HookContext, err error) error) Hook {
	return &HookFunc{
		AfterFn: func(ctx HookContext, result any, err error) (any, error) {
			if err != nil {
				return result, handler(ctx, err)
			}
			return result, nil
		},
	}
}

// hookChain manages multiple hooks for sequential execution.
type hookChain []Hook

// executeBefore runs all Before hooks in order, stopping at first error.
// Recovers from panics to prevent a misbehaving hook from crashing the processor.
func (hc hookChain) executeBefore(ctx HookContext) error {
	for _, h := range hc {
		var hookErr error
		func() {
			defer func() {
				if r := recover(); r != nil {
					hookErr = fmt.Errorf("hook panicked: %v", r)
				}
			}()
			hookErr = h.Before(ctx)
		}()
		if hookErr != nil {
			return hookErr
		}
	}
	return nil
}

// executeAfter runs all After hooks in reverse order.
// Recovers from panics to prevent a misbehaving hook from crashing the processor.
// Logs recovered panics so they are not silently lost.
func (hc hookChain) executeAfter(ctx HookContext, result any, err error) (any, error) {
	for i := len(hc) - 1; i >= 0; i-- {
		func() {
			defer func() {
				if r := recover(); r != nil {
					slog.Error("hook panicked in After",
						slog.String("operation", ctx.Operation),
						slog.String("path", ctx.Path),
						slog.Int("hook_index", i),
						slog.Any("panic", r),
					)
				}
			}()
			result, err = hc[i].After(ctx, result, err)
		}()
	}
	return result, err
}

// =============================================================================
// 6. Path Parser Interface (Breaks Circular Dependency)
// =============================================================================

// PathParser parses path strings into segments.
// Implement to provide custom path syntax support.
type PathParser interface {
	// ParsePath parses a path string into segments.
	// Returns an error if path is syntactically invalid.
	ParsePath(path string) ([]PathSegment, error)
}

// =============================================================================
// Convenience Functions for Path Segments
// These delegate to internal package constructors.
// =============================================================================

// newPropertySegment creates a property access segment.
func newPropertySegment(key string) PathSegment {
	return internal.NewPropertySegment(key)
}

// newArrayIndexSegment creates an array index access segment.
func newArrayIndexSegment(index int) PathSegment {
	return internal.NewArrayIndexSegment(index)
}

// newArraySliceSegment creates an array slice segment.
func newArraySliceSegment(start, end, step int, hasStart, hasEnd, hasStep bool) PathSegment {
	return internal.NewArraySliceSegment(start, end, step, hasStart, hasEnd, hasStep)
}

// newWildcardSegment creates a wildcard segment.
func newWildcardSegment() PathSegment {
	return internal.NewWildcardSegment()
}

// newExtractSegment creates an extraction segment.
func newExtractSegment(key string, flat bool) PathSegment {
	return internal.NewExtractSegmentWithFlat(key, flat)
}

// newAppendSegment creates an append segment.
func newAppendSegment() PathSegment {
	return internal.PathSegment{
		Type: internal.AppendSegment,
	}
}
