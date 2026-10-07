package json

import (
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"
)

// ============================================================================
// mock types for hook testing
// ============================================================================

// mockLogger records log calls for verification.
type mockLogger struct {
	mu    sync.Mutex
	calls []mockLogEntry
}

type mockLogEntry struct {
	msg  string
	args []any
}

func (m *mockLogger) Info(msg string, args ...any) {
	m.mu.Lock()
	m.calls = append(m.calls, mockLogEntry{msg: msg, args: args})
	m.mu.Unlock()
}

func (m *mockLogger) last() (mockLogEntry, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.calls) == 0 {
		return mockLogEntry{}, false
	}
	return m.calls[len(m.calls)-1], true
}

func (m *mockLogger) count() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.calls)
}

// mockRecorder records timing calls for verification.
type mockRecorder struct {
	mu      sync.Mutex
	records []mockTimingRecord
}

type mockTimingRecord struct {
	op       string
	duration time.Duration
}

func (m *mockRecorder) Record(op string, d time.Duration) {
	m.mu.Lock()
	m.records = append(m.records, mockTimingRecord{op: op, duration: d})
	m.mu.Unlock()
}

// orderRecord tracks hook execution order.
type orderRecord struct {
	name  string
	phase string // "before" or "after"
}

// ============================================================================
// TestAddHook
// ============================================================================

func TestAddHook(t *testing.T) {
	t.Run("HookFiresDuringGet", func(t *testing.T) {
		p, err := New()
		if err != nil {
			t.Fatalf("New() error: %v", err)
		}
		defer p.Close()

		var beforeCalled bool
		var afterCalled bool

		p.AddHook(&HookFunc{
			BeforeFn: func(ctx HookContext) error {
				beforeCalled = true
				return nil
			},
			AfterFn: func(ctx HookContext, result any, err error) (any, error) {
				afterCalled = true
				return result, err
			},
		})

		// The hook must fire during a REAL Get, not just when driven through
		// hookChain manually (that path is covered by TestHookChainExecution).
		if _, err := p.Get(`{"key":"value"}`, "key"); err != nil {
			t.Fatalf("Get error: %v", err)
		}
		if !beforeCalled {
			t.Error("Before hook did not fire during Get")
		}
		if !afterCalled {
			t.Error("After hook did not fire during Get")
		}
	})

	t.Run("MultipleHooksExecuteInOrder", func(t *testing.T) {
		p, err := New()
		if err != nil {
			t.Fatalf("New() error: %v", err)
		}
		defer p.Close()

		var order []orderRecord

		// Add hooks in order: first, second, third.
		p.AddHook(&HookFunc{
			BeforeFn: func(ctx HookContext) error {
				order = append(order, orderRecord{"first", "before"})
				return nil
			},
			AfterFn: func(ctx HookContext, result any, err error) (any, error) {
				order = append(order, orderRecord{"first", "after"})
				return result, err
			},
		})
		p.AddHook(&HookFunc{
			BeforeFn: func(ctx HookContext) error {
				order = append(order, orderRecord{"second", "before"})
				return nil
			},
			AfterFn: func(ctx HookContext, result any, err error) (any, error) {
				order = append(order, orderRecord{"second", "after"})
				return result, err
			},
		})
		p.AddHook(&HookFunc{
			BeforeFn: func(ctx HookContext) error {
				order = append(order, orderRecord{"third", "before"})
				return nil
			},
			AfterFn: func(ctx HookContext, result any, err error) (any, error) {
				order = append(order, orderRecord{"third", "after"})
				return result, err
			},
		})

		hc := hookChain(p.hooks)
		ctx := HookContext{Operation: "get", StartTime: time.Now()}

		_ = hc.executeBefore(ctx)
		_, _ = hc.executeAfter(ctx, nil, nil)

		// Before: first, second, third (in order)
		// After: third, second, first (reverse order)
		expected := []orderRecord{
			{"first", "before"},
			{"second", "before"},
			{"third", "before"},
			{"third", "after"},
			{"second", "after"},
			{"first", "after"},
		}

		if len(order) != len(expected) {
			t.Fatalf("expected %d hook calls, got %d", len(expected), len(order))
		}
		for i, exp := range expected {
			if order[i] != exp {
				t.Errorf("hook call %d: expected %v, got %v", i, exp, order[i])
			}
		}
	})

	t.Run("NilProcessorDoesNotPanic", func(t *testing.T) {
		var p *Processor
		// Should not panic.
		p.AddHook(&HookFunc{})
	})

	t.Run("ConcurrentAddHook", func(t *testing.T) {
		p, err := New()
		if err != nil {
			t.Fatalf("New() error: %v", err)
		}
		defer p.Close()

		const goroutines = 50
		var wg sync.WaitGroup
		wg.Add(goroutines)

		for i := 0; i < goroutines; i++ {
			go func() {
				defer wg.Done()
				p.AddHook(&HookFunc{
					BeforeFn: func(ctx HookContext) error { return nil },
					AfterFn:  func(ctx HookContext, result any, err error) (any, error) { return result, err },
				})
			}()
		}

		wg.Wait()

		if len(p.hooks) != goroutines {
			t.Errorf("expected %d hooks, got %d", goroutines, len(p.hooks))
		}
	})
}

// ============================================================================
// TestLoggingHook
// ============================================================================

func TestLoggingHook(t *testing.T) {
	t.Run("BeforeLogsOperationStart", func(t *testing.T) {
		logger := &mockLogger{}
		hook := LoggingHook(logger)

		ctx := HookContext{
			Operation: "get",
			Path:      "users[0].name",
			StartTime: time.Now(),
		}

		if err := hook.Before(ctx); err != nil {
			t.Errorf("Before returned unexpected error: %v", err)
		}

		if logger.count() != 1 {
			t.Fatalf("expected 1 log call, got %d", logger.count())
		}

		entry, ok := logger.last()
		if !ok {
			t.Fatal("expected a log entry")
		}
		if entry.msg != "operation starting" {
			t.Errorf("expected msg 'operation starting', got %q", entry.msg)
		}
	})

	t.Run("AfterSuccessLogsCompletion", func(t *testing.T) {
		logger := &mockLogger{}
		hook := LoggingHook(logger)

		ctx := HookContext{
			Operation: "set",
			Path:      "users[0].name",
			StartTime: time.Now(),
		}

		result, err := hook.After(ctx, "result", nil)
		if err != nil {
			t.Errorf("After returned unexpected error: %v", err)
		}
		if result != "result" {
			t.Errorf("expected result 'result', got %v", result)
		}

		// Logger should have been called from After (Before not called in this sub-test)
		if logger.count() != 1 {
			t.Fatalf("expected 1 log call, got %d", logger.count())
		}

		entry, ok := logger.last()
		if !ok {
			t.Fatal("expected a log entry")
		}
		if entry.msg != "operation completed" {
			t.Errorf("expected msg 'operation completed', got %q", entry.msg)
		}
	})

	t.Run("AfterErrorLogsError", func(t *testing.T) {
		logger := &mockLogger{}
		hook := LoggingHook(logger)

		ctx := HookContext{
			Operation: "delete",
			Path:      "users[0]",
			StartTime: time.Now(),
		}

		testErr := errors.New("path not found")
		result, err := hook.After(ctx, nil, testErr)

		if result != nil {
			t.Errorf("expected nil result, got %v", result)
		}
		if err != testErr {
			t.Errorf("expected error to be passed through, got %v", err)
		}

		entry, ok := logger.last()
		if !ok {
			t.Fatal("expected a log entry")
		}
		if entry.msg != "operation completed" {
			t.Errorf("expected msg 'operation completed', got %q", entry.msg)
		}
		// GEN-001 P1: the error is logged SANITIZED (as a string, like the
		// library's own logError redaction), not as the raw error value.
		found := false
		for i := 0; i < len(entry.args)-1; i++ {
			if entry.args[i] == "error" && entry.args[i+1] == testErr.Error() {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected 'error' key with sanitized error %q in log args", testErr.Error())
		}
	})

	t.Run("SensitivePathRedacted", func(t *testing.T) {
		// GEN-001 P1: LoggingHook must sanitize the path exactly like the
		// library's own logging — a path through a credential field must not
		// reach logs verbatim.
		logger := &mockLogger{}
		hook := LoggingHook(logger)

		ctx := HookContext{
			Operation: "get",
			Path:      "users[0].auth.token",
			StartTime: time.Now(),
		}
		if err := hook.Before(ctx); err != nil {
			t.Fatalf("Before: %v", err)
		}
		entry, ok := logger.last()
		if !ok {
			t.Fatal("expected a log entry")
		}
		for i := 0; i < len(entry.args)-1; i++ {
			if entry.args[i] == "path" {
				if entry.args[i+1] != "[REDACTED_PATH]" {
					t.Errorf("path arg = %v, want [REDACTED_PATH]", entry.args[i+1])
				}
				return
			}
		}
		t.Error("expected a 'path' key in log args")
	})
}

// ============================================================================
// TestTimingHook
// ============================================================================

func TestTimingHook(t *testing.T) {
	t.Run("BeforeReturnsNilNoBeforeFn", func(t *testing.T) {
		recorder := &mockRecorder{}
		hook := TimingHook(recorder)

		ctx := HookContext{
			Operation: "get",
			Path:      "test",
		}

		if err := hook.Before(ctx); err != nil {
			t.Errorf("Before should return nil when BeforeFn is not set, got: %v", err)
		}
	})

	t.Run("AfterRecordsDuration", func(t *testing.T) {
		recorder := &mockRecorder{}
		hook := TimingHook(recorder)

		ctx := HookContext{
			Operation: "set",
			Path:      "test",
			StartTime: time.Now().Add(-50 * time.Millisecond), // simulate 50ms elapsed
		}

		result, err := hook.After(ctx, "value", nil)
		if err != nil {
			t.Errorf("After returned unexpected error: %v", err)
		}
		if result != "value" {
			t.Errorf("expected result 'value', got %v", result)
		}

		if len(recorder.records) != 1 {
			t.Fatalf("expected 1 record, got %d", len(recorder.records))
		}

		rec := recorder.records[0]
		if rec.op != "set" {
			t.Errorf("expected op 'set', got %q", rec.op)
		}
		if rec.duration <= 0 {
			t.Errorf("expected positive duration, got %v", rec.duration)
		}
	})

	t.Run("AfterPassesErrorThrough", func(t *testing.T) {
		recorder := &mockRecorder{}
		hook := TimingHook(recorder)

		ctx := HookContext{
			Operation: "delete",
			Path:      "missing",
			StartTime: time.Now(),
		}

		testErr := errors.New("not found")
		result, err := hook.After(ctx, nil, testErr)

		if result != nil {
			t.Errorf("expected nil result, got %v", result)
		}
		if err != testErr {
			t.Errorf("expected error to pass through, got %v", err)
		}

		// Timing should still be recorded even on error.
		if len(recorder.records) != 1 {
			t.Fatalf("expected 1 record even on error, got %d", len(recorder.records))
		}
	})
}

// ============================================================================
// TestValidationHook
// ============================================================================

func TestValidationHook(t *testing.T) {
	t.Run("ValidatorAccepts_ReturnsNil", func(t *testing.T) {
		validator := func(jsonStr, path string) error {
			return nil // accepts everything
		}
		hook := ValidationHook(validator)

		ctx := HookContext{
			Operation: "set",
			JSONStr:   `{"a":1}`,
			Path:      "a",
		}

		if err := hook.Before(ctx); err != nil {
			t.Errorf("Before should return nil when validator accepts, got: %v", err)
		}
	})

	t.Run("ValidatorRejects_ReturnsError", func(t *testing.T) {
		validationErr := errors.New("JSON too large")
		validator := func(jsonStr, path string) error {
			return validationErr
		}
		hook := ValidationHook(validator)

		ctx := HookContext{
			Operation: "set",
			JSONStr:   `{"a":1}`,
			Path:      "a",
		}

		err := hook.Before(ctx)
		if err == nil {
			t.Fatal("Before should return error when validator rejects")
		}
		if err != validationErr {
			t.Errorf("expected validationErr, got: %v", err)
		}
	})

	t.Run("ValidatorReceivesCorrectArguments", func(t *testing.T) {
		var receivedJSON, receivedPath string
		validator := func(jsonStr, path string) error {
			receivedJSON = jsonStr
			receivedPath = path
			return nil
		}
		hook := ValidationHook(validator)

		ctx := HookContext{
			Operation: "get",
			JSONStr:   `{"x":42}`,
			Path:      "x",
		}

		_ = hook.Before(ctx)

		if receivedJSON != `{"x":42}` {
			t.Errorf("expected JSON %q, got %q", `{"x":42}`, receivedJSON)
		}
		if receivedPath != "x" {
			t.Errorf("expected path %q, got %q", "x", receivedPath)
		}
	})

	t.Run("AfterReturnsOriginalResultAndError", func(t *testing.T) {
		hook := ValidationHook(func(jsonStr, path string) error { return nil })

		ctx := HookContext{Operation: "set", StartTime: time.Now()}
		result, err := hook.After(ctx, "original", nil)

		if result != "original" {
			t.Errorf("expected 'original' result, got %v", result)
		}
		if err != nil {
			t.Errorf("expected nil error, got %v", err)
		}

		testErr := errors.New("some error")
		result, err = hook.After(ctx, "data", testErr)
		if result != "data" {
			t.Errorf("expected 'data' result, got %v", result)
		}
		if err != testErr {
			t.Errorf("expected error to pass through, got %v", err)
		}
	})
}

// ============================================================================
// TestErrorHook
// ============================================================================

func TestErrorHook(t *testing.T) {
	t.Run("AfterNilError_HandlerNotCalled", func(t *testing.T) {
		var handlerCalled bool
		handler := func(ctx HookContext, err error) error {
			handlerCalled = true
			return err
		}
		hook := ErrorHook(handler)

		ctx := HookContext{Operation: "get", StartTime: time.Now()}
		result, err := hook.After(ctx, "value", nil)

		if handlerCalled {
			t.Error("handler should NOT be called when err is nil")
		}
		if result != "value" {
			t.Errorf("expected 'value', got %v", result)
		}
		if err != nil {
			t.Errorf("expected nil error, got %v", err)
		}
	})

	t.Run("AfterError_HandlerCalled", func(t *testing.T) {
		var handlerCalled bool
		var receivedErr error
		handler := func(ctx HookContext, err error) error {
			handlerCalled = true
			receivedErr = err
			return err
		}
		hook := ErrorHook(handler)

		ctx := HookContext{Operation: "delete", Path: "x", StartTime: time.Now()}
		testErr := errors.New("not found")
		result, err := hook.After(ctx, nil, testErr)

		if !handlerCalled {
			t.Error("handler should be called when err is non-nil")
		}
		if receivedErr != testErr {
			t.Errorf("handler received err %v, expected %v", receivedErr, testErr)
		}
		if result != nil {
			t.Errorf("expected nil result, got %v", result)
		}
		if err != testErr {
			t.Errorf("expected error to pass through, got %v", err)
		}
	})

	t.Run("AfterError_HandlerTransformsError", func(t *testing.T) {
		transformedErr := errors.New("transformed")
		handler := func(ctx HookContext, err error) error {
			return transformedErr
		}
		hook := ErrorHook(handler)

		ctx := HookContext{Operation: "set", StartTime: time.Now()}
		originalErr := errors.New("original")
		_, err := hook.After(ctx, "result", originalErr)

		if err != transformedErr {
			t.Errorf("expected transformed error, got %v", err)
		}
	})

	t.Run("AfterError_HandlerReturnsNil_SuppressesError", func(t *testing.T) {
		handler := func(ctx HookContext, err error) error {
			return nil // suppress error
		}
		hook := ErrorHook(handler)

		ctx := HookContext{Operation: "get", StartTime: time.Now()}
		originalErr := errors.New("should be suppressed")
		result, err := hook.After(ctx, "data", originalErr)

		if result != "data" {
			t.Errorf("expected 'data' result, got %v", result)
		}
		if err != nil {
			t.Errorf("expected nil error (suppressed), got %v", err)
		}
	})
}

// ============================================================================
// TestHookChainExecution
// ============================================================================

func TestHookChainExecution(t *testing.T) {
	t.Run("ExecuteBeforeStopsAtFirstError", func(t *testing.T) {
		blockErr := errors.New("blocked")
		var secondCalled bool

		hc := hookChain{
			&HookFunc{
				BeforeFn: func(ctx HookContext) error { return blockErr },
			},
			&HookFunc{
				BeforeFn: func(ctx HookContext) error {
					secondCalled = true
					return nil
				},
			},
		}

		err := hc.executeBefore(HookContext{})
		if err != blockErr {
			t.Errorf("expected blockErr, got %v", err)
		}
		if secondCalled {
			t.Error("second hook should not be called after first returns error")
		}
	})

	t.Run("ExecuteAfterReverseOrder", func(t *testing.T) {
		var order []string
		hc := hookChain{
			&HookFunc{
				AfterFn: func(ctx HookContext, result any, err error) (any, error) {
					order = append(order, "first")
					return result, err
				},
			},
			&HookFunc{
				AfterFn: func(ctx HookContext, result any, err error) (any, error) {
					order = append(order, "second")
					return result, err
				},
			},
			&HookFunc{
				AfterFn: func(ctx HookContext, result any, err error) (any, error) {
					order = append(order, "third")
					return result, err
				},
			},
		}

		_, _ = hc.executeAfter(HookContext{}, nil, nil)

		expected := []string{"third", "second", "first"}
		if len(order) != len(expected) {
			t.Fatalf("expected %d calls, got %d", len(expected), len(order))
		}
		for i, exp := range expected {
			if order[i] != exp {
				t.Errorf("after call %d: expected %q, got %q", i, exp, order[i])
			}
		}
	})

	t.Run("HookFuncNilBeforeFnReturnsNil", func(t *testing.T) {
		h := &HookFunc{} // both BeforeFn and AfterFn are nil
		if err := h.Before(HookContext{}); err != nil {
			t.Errorf("nil BeforeFn should return nil, got %v", err)
		}
	})

	t.Run("HookFuncNilAfterFnReturnsOriginal", func(t *testing.T) {
		h := &HookFunc{}
		result, err := h.After(HookContext{}, "test", nil)
		if result != "test" {
			t.Errorf("nil AfterFn should return original result, got %v", result)
		}
		if err != nil {
			t.Errorf("nil AfterFn should return original error, got %v", err)
		}
	})
}

// ============================================================================
// TestHookIntegrationWithProcessor
// ============================================================================

func TestHookIntegrationWithProcessor(t *testing.T) {
	t.Run("MultipleHookTypesOnProcessor", func(t *testing.T) {
		p, err := New()
		if err != nil {
			t.Fatalf("New() error: %v", err)
		}
		defer p.Close()

		logger := &mockLogger{}
		recorder := &mockRecorder{}

		p.AddHook(LoggingHook(logger))
		p.AddHook(TimingHook(recorder))
		p.AddHook(ErrorHook(func(ctx HookContext, err error) error { return err }))

		if len(p.hooks) != 3 {
			t.Fatalf("expected 3 hooks, got %d", len(p.hooks))
		}

		hc := hookChain(p.hooks)
		ctx := HookContext{
			Operation: "get",
			Path:      "test",
			JSONStr:   `{"test":1}`,
			StartTime: time.Now(),
		}

		_ = hc.executeBefore(ctx)
		_, _ = hc.executeAfter(ctx, 1, nil)

		// LoggingHook.Before logs "operation starting"
		// After runs in reverse: ErrorHook, TimingHook, LoggingHook
		if logger.count() != 2 {
			t.Errorf("expected 2 log entries, got %d", logger.count())
		}
		if len(recorder.records) != 1 {
			t.Errorf("expected 1 timing record, got %d", len(recorder.records))
		}
	})

	t.Run("ValidationHookAbortsBeforeOtherHooks", func(t *testing.T) {
		p, err := New()
		if err != nil {
			t.Fatalf("New() error: %v", err)
		}
		defer p.Close()

		var secondBeforeCalled bool
		validationErr := errors.New("rejected")

		p.AddHook(ValidationHook(func(jsonStr, path string) error {
			return validationErr
		}))
		p.AddHook(&HookFunc{
			BeforeFn: func(ctx HookContext) error {
				secondBeforeCalled = true
				return nil
			},
		})

		hc := hookChain(p.hooks)
		err = hc.executeBefore(HookContext{JSONStr: `{}`, Path: "x"})
		if err != validationErr {
			t.Errorf("expected validationErr, got %v", err)
		}
		if secondBeforeCalled {
			t.Error("second hook Before should not be called after validation abort")
		}
	})
}

// helper: parse JSON string into map for structural comparison.
func mustParseMap(t *testing.T, jsonStr string) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal([]byte(jsonStr), &m); err != nil {
		t.Fatalf("invalid JSON %q: %v", jsonStr, err)
	}
	return m
}

// helper: check that a JSON string contains a given substring.
func assertContains(t *testing.T, jsonStr, substr string) {
	t.Helper()
	if !strings.Contains(jsonStr, substr) {
		t.Errorf("expected JSON to contain %q, got %q", substr, jsonStr)
	}
}
