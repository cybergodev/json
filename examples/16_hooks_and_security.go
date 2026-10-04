//go:build example

package main

import (
	"errors"
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

	// Perform operations through the processor — hooks fire automatically.
	// Results are discarded; the point is to exercise the hook.
	testData := `{"user": {"name": "Alice"}, "admin": true}`
	_, _ = processor.Get(testData, "user.name") // best-effort: exercising the hook
	_, _ = processor.Get(testData, "admin")     // best-effort: exercising the hook

	fmt.Printf("   Hook state after processor operations:\n")
	fmt.Printf("   - Before calls: %d\n", hook.beforeCount)
	fmt.Printf("   - After calls:  %d\n", hook.afterCount)
	fmt.Printf("   - Operations tracked: %v\n", hook.operations)
}

func demonstrateHookFunc() {
	fmt.Println("\n2. HookFunc Adapter (function-style hooks)")
	fmt.Println("--------------------------------------------")

	// HookFunc turns plain functions into a Hook — set only BeforeFn, only
	// AfterFn, or both (unset ones are no-ops). Register it with AddHook and it
	// fires for real operations: a Before error aborts the operation.
	processor, _ := json.New(json.DefaultConfig()) // OK: preset config always valid
	defer processor.Close()

	var calls []string
	processor.AddHook(&json.HookFunc{
		BeforeFn: func(ctx json.HookContext) error {
			calls = append(calls, "before:"+ctx.Path)
			if ctx.Path == "admin" {
				return fmt.Errorf("blocked: cannot access admin path")
			}
			return nil
		},
		AfterFn: func(ctx json.HookContext, result any, err error) (any, error) {
			calls = append(calls, "after:"+ctx.Path)
			return result, err
		},
	})

	data := `{"user":"Alice","admin":true}`

	if _, err := processor.Get(data, "user"); err != nil {
		fmt.Printf("   Get('user'):  error: %v\n", err)
	} else {
		fmt.Println("   Get('user'):  ok (Before returned nil)")
	}

	if _, err := processor.Get(data, "admin"); err != nil {
		fmt.Printf("   Get('admin'): error: %v (aborted by Before hook)\n", err)
	} else {
		fmt.Println("   Get('admin'): ok")
	}

	fmt.Printf("   Hook calls recorded: %v\n", calls)
}

// exampleLogger implements the Info method for LoggingHook. In production this
// would write to a real logger; here it prints so the hook's effect is visible.
type exampleLogger struct{}

func (l *exampleLogger) Info(msg string, args ...any) {
	fmt.Printf("   [log] %s %v\n", msg, args)
}

// exampleRecorder implements the Record method for TimingHook.
type exampleRecorder struct{}

func (r *exampleRecorder) Record(op string, duration time.Duration) {
	fmt.Printf("   [timing] %s took %v\n", op, duration)
}

func demonstrateConvenienceHooks() {
	fmt.Println("\n3. Convenience Hook Constructors")
	fmt.Println("----------------------------------")

	processor, _ := json.New(json.DefaultConfig()) // OK: preset config always valid
	defer processor.Close()

	// Register the four convenience hooks. LoggingHook and TimingHook print via
	// the exampleLogger/exampleRecorder above so their effect is visible.
	processor.AddHook(json.LoggingHook(&exampleLogger{}))
	processor.AddHook(json.TimingHook(&exampleRecorder{}))
	processor.AddHook(json.ValidationHook(func(jsonStr, path string) error {
		if len(jsonStr) > 1_000_000 { // 1MB ceiling for any single operation
			return fmt.Errorf("JSON too large: %d bytes", len(jsonStr))
		}
		return nil
	}))
	processor.AddHook(json.ErrorHook(func(ctx json.HookContext, err error) error {
		fmt.Printf("   [error] op=%s err=%v\n", ctx.Operation, err)
		return err
	}))
	fmt.Println("   Registered LoggingHook, TimingHook, ValidationHook, ErrorHook")

	// Run an operation through the processor so the hooks actually fire.
	fmt.Println("   Running Get through the hooked processor:")
	data := `{"user":"Alice"}`
	if _, err := processor.Get(data, "user"); err != nil {
		fmt.Printf("   Get error: %v\n", err)
	} else {
		fmt.Println("   Get completed (see hook output above)")
	}

	// Config.Hooks: hooks can be pre-wired in the Config passed to New instead
	// of added one-by-one via AddHook after construction.
	fmt.Println("\n   Constructing a processor with Config.Hooks (pre-wired):")
	cfg := json.DefaultConfig()
	var beforeOps []string
	cfg.Hooks = []json.Hook{
		&json.HookFunc{BeforeFn: func(ctx json.HookContext) error {
			beforeOps = append(beforeOps, ctx.Operation)
			return nil
		}},
	}
	preWired, _ := json.New(cfg) // OK: DefaultConfig-derived, always valid
	defer preWired.Close()
	_, _ = preWired.Get(data, "user") // best-effort: exercising the pre-wired hook
	fmt.Printf("   Pre-wired hook saw operations: %v\n", beforeOps)
}

func demonstrateSecurityPatterns() {
	fmt.Println("\n4. Security Patterns")
	fmt.Println("----------------------")

	// ListDangerousPatterns returns USER-REGISTERED patterns only. The library's
	// built-in dangerous and critical patterns are always active and are
	// intentionally not exposed through this list, so an empty result here does
	// NOT mean validation is off — the built-ins still run on every input.
	defaults := json.ListDangerousPatterns()
	fmt.Printf("   User-registered patterns: %d (built-in patterns always active)\n", len(defaults))
	for _, p := range defaults {
		fmt.Printf("   - [%s] %s\n", p.Level, p.Name)
	}

	// Filter the user-registered patterns for the critical level.
	var critical []json.DangerousPattern
	for _, p := range defaults {
		if p.Level == json.PatternLevelCritical {
			critical = append(critical, p)
		}
	}
	fmt.Printf("\n   User-registered critical patterns: %d\n", len(critical))
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
	fmt.Printf("   User-registered patterns after registration: %d\n", len(all))

	// Unregister the custom pattern
	json.UnregisterDangerousPattern("eval(")
	fmt.Println("   Unregistered custom pattern")

	all = json.ListDangerousPatterns()
	fmt.Printf("   User-registered patterns after removal: %d\n", len(all))

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

	// A registered pattern actually blocks input. RegisterDangerousPattern
	// (above) is process-global; Config.AddDangerousPattern scopes a pattern
	// to a single processor, leaving the global registry untouched.
	patCfg := json.DefaultConfig()
	patCfg.AddDangerousPattern(json.DangerousPattern{
		Pattern: "eval(",
		Name:    "JavaScript eval injection",
		Level:   json.PatternLevelCritical,
	})
	patProc, _ := json.New(patCfg) // OK: DefaultConfig-derived, always valid
	defer patProc.Close()

	_, err = patProc.Get(`{"code": "eval(alert(1))"}`, "code")
	fmt.Printf("\n   Per-Config pattern blocks 'eval(': %v\n", err)
	fmt.Printf("   classified as ErrSecurityViolation: %t\n",
		errors.Is(err, json.ErrSecurityViolation))
}
