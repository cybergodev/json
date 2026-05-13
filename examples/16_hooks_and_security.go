//go:build example

package main

import (
	"fmt"
	"time"

	"github.com/cybergodev/json"
)

// Hooks and Security Patterns Example
//
// This example demonstrates the hook system for operation interception
// and security pattern management for content validation.
//
// Topics covered:
// - Hook interface and HookFunc adapter
// - Convenience constructors (LoggingHook, TimingHook, ErrorHook, ValidationHook)
// - AddHook for registering hooks on a processor
// - Security pattern registration and management
//
// Run: go run -tags=example examples/16_hooks_and_security.go

func main() {
	fmt.Println("JSON Library - Hooks and Security")
	fmt.Println("==================================")

	// 1. HOOK INTERFACE
	demonstrateHookInterface()

	// 2. HOOKFUNC ADAPTER
	demonstrateHookFunc()

	// 3. CONVENIENCE HOOK CONSTRUCTORS
	demonstrateConvenienceHooks()

	// 4. SECURITY PATTERNS
	demonstrateSecurityPatterns()

	fmt.Println("\nHooks and security examples complete!")
}

// countingHook implements the Hook interface directly
type countingHook struct {
	beforeCount, afterCount int
	operations              []string
}

func (h *countingHook) Before(ctx json.HookContext) error {
	h.beforeCount++
	h.operations = append(h.operations, ctx.Operation)
	return nil
}

func (h *countingHook) After(ctx json.HookContext, result any, err error) (any, error) {
	h.afterCount++
	return result, err
}

func demonstrateHookInterface() {
	fmt.Println("1. Hook Interface (custom Hook implementation)")
	fmt.Println("-------------------------------------------------")

	processor, _ := json.New(json.DefaultConfig()) // OK: preset config always valid
	defer processor.Close()

	// Add a custom hook that implements the Hook interface.
	// Hooks are automatically invoked by processor operations (Get, Set, Delete, etc.)
	hook := &countingHook{}
	processor.AddHook(hook)

	// Perform operations through the processor — hooks fire automatically
	testData := `{"user": {"name": "Alice"}, "admin": true}`
	_, _ = processor.Get(testData, "user.name")
	_, _ = processor.Get(testData, "admin")

	fmt.Printf("   Hook state after processor operations:\n")
	fmt.Printf("   - Before calls: %d\n", hook.beforeCount)
	fmt.Printf("   - After calls:  %d\n", hook.afterCount)
	fmt.Printf("   - Operations tracked: %v\n", hook.operations)
}

func demonstrateHookFunc() {
	fmt.Println("\n2. HookFunc Adapter (function-style hooks)")
	fmt.Println("--------------------------------------------")

	// HookFunc allows creating hooks from plain functions.
	// Only set the functions you need - unset ones are no-ops.
	// Below we demonstrate each hook function's behavior directly.
	// In practice, register via processor.AddHook() and they fire automatically.

	// Before-only hook (validation)
	validateHook := &json.HookFunc{
		BeforeFn: func(ctx json.HookContext) error {
			fmt.Printf("   [Before] op=%s path=%s\n", ctx.Operation, ctx.Path)
			if ctx.Path == "admin" {
				return fmt.Errorf("blocked: cannot access admin path")
			}
			return nil
		},
	}

	ctx := json.HookContext{Operation: "get", Path: "user.name"}
	err := validateHook.Before(ctx)
	fmt.Printf("   Before('user.name'): err=%v\n", err)

	ctx2 := json.HookContext{Operation: "get", Path: "admin"}
	err = validateHook.Before(ctx2)
	fmt.Printf("   Before('admin'): err=%v\n", err)

	// After-only hook (result logging)
	afterHook := &json.HookFunc{
		AfterFn: func(ctx json.HookContext, result any, err error) (any, error) {
			status := "ok"
			if err != nil {
				status = "error: " + err.Error()
			}
			fmt.Printf("   [After] op=%s result=%v status=%s\n", ctx.Operation, result, status)
			return result, err
		},
	}

	afterHook.After(json.HookContext{Operation: "get"}, "Alice", nil)
	afterHook.After(json.HookContext{Operation: "get"}, nil, fmt.Errorf("not found"))

	// Both Before and After (timing)
	timingHook := &json.HookFunc{
		BeforeFn: func(ctx json.HookContext) error {
			fmt.Printf("   [Before] %s started\n", ctx.Operation)
			return nil
		},
		AfterFn: func(ctx json.HookContext, result any, err error) (any, error) {
			elapsed := time.Since(ctx.StartTime)
			fmt.Printf("   [After] %s completed in %v\n", ctx.Operation, elapsed)
			return result, err
		},
	}

	startTime := time.Now()
	ctx3 := json.HookContext{Operation: "set", StartTime: startTime}
	timingHook.Before(ctx3)
	result, _ := timingHook.After(ctx3, `{"updated": true}`, nil)
	fmt.Printf("   Timing hook result: %v\n", result)
}

// exampleLogger implements the Info method for LoggingHook
type exampleLogger struct{}

func (l *exampleLogger) Info(msg string, args ...any) {
	// In production, this would write to a real logger
}

// exampleRecorder implements the Record method for TimingHook
type exampleRecorder struct{}

func (r *exampleRecorder) Record(op string, duration time.Duration) {
	// In production, this would record to a metrics system
}

func demonstrateConvenienceHooks() {
	fmt.Println("\n3. Convenience Hook Constructors")
	fmt.Println("----------------------------------")

	processor, _ := json.New(json.DefaultConfig()) // OK: preset config always valid
	defer processor.Close()

	// LoggingHook - logs operation start/completion
	loggingHook := json.LoggingHook(&exampleLogger{})
	processor.AddHook(loggingHook)
	fmt.Println("   LoggingHook registered")

	// TimingHook - records operation duration
	timingHook := json.TimingHook(&exampleRecorder{})
	processor.AddHook(timingHook)
	fmt.Println("   TimingHook registered")

	// ValidationHook - validates input before operations
	validationHook := json.ValidationHook(func(jsonStr, path string) error {
		if len(jsonStr) > 1_000_000 {
			return fmt.Errorf("JSON too large: %d bytes", len(jsonStr))
		}
		return nil
	})
	processor.AddHook(validationHook)
	fmt.Println("   ValidationHook registered")

	// ErrorHook - intercepts and can transform errors
	errorHook := json.ErrorHook(func(ctx json.HookContext, err error) error {
		fmt.Printf("   [ErrorHook] op=%s err=%v\n", ctx.Operation, err)
		return err
	})
	processor.AddHook(errorHook)
	fmt.Println("   ErrorHook registered")

	fmt.Println("\n   Hooks are invoked by the processor's internal hook chain")
	fmt.Println("   during operations like Get, Set, Delete, etc.")
}

func demonstrateSecurityPatterns() {
	fmt.Println("\n4. Security Patterns")
	fmt.Println("----------------------")

	// List default patterns
	defaults := json.ListDangerousPatterns()
	fmt.Printf("   Default security patterns: %d\n", len(defaults))
	for _, p := range defaults {
		fmt.Printf("   - [%s] %s\n", p.Level, p.Name)
	}

	// List critical patterns (filter from all patterns)
	var critical []json.DangerousPattern
	for _, p := range defaults {
		if p.Level == json.PatternLevelCritical {
			critical = append(critical, p)
		}
	}
	fmt.Printf("\n   Critical patterns: %d\n", len(critical))
	for _, p := range critical {
		fmt.Printf("   - %s (pattern: %s)\n", p.Name, p.Pattern)
	}

	// Register a custom dangerous pattern
	customPattern := json.DangerousPattern{
		Pattern: "eval(",
		Name:    "JavaScript eval injection",
		Level:   json.PatternLevelCritical,
	}
	json.RegisterDangerousPattern(customPattern)
	fmt.Println("\n   Registered custom pattern: 'eval('")

	// List all patterns after registration
	all := json.ListDangerousPatterns()
	fmt.Printf("   Total patterns after registration: %d\n", len(all))

	// Unregister the custom pattern
	json.UnregisterDangerousPattern("eval(")
	fmt.Println("   Unregistered custom pattern")

	all = json.ListDangerousPatterns()
	fmt.Printf("   Total patterns after removal: %d\n", len(all))

	// Demonstrate SecurityConfig for secure processing
	fmt.Println("\n   Using SecurityConfig processor:")
	secProcessor, _ := json.New(json.SecurityConfig()) // OK: preset config always valid
	defer secProcessor.Close()

	safeJSON := `{"user": "Alice", "action": "login"}`
	result, err := secProcessor.Get(safeJSON, "user")
	if err != nil {
		fmt.Printf("   Error: %v\n", err)
	} else {
		fmt.Printf("   Safe data processed: %v\n", result)
	}
}
