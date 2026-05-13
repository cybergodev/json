//go:build example

package main

import (
	"fmt"
	"time"

	"github.com/cybergodev/json"
)

// Advanced Patterns Example
//
// This example demonstrates advanced patterns for high-performance
// and safe JSON operations using pre-parsed data, compiled paths,
// and safe access patterns.
//
// Topics covered:
// - SafeGet and AccessResult for null-safe access
// - PreParse for reusing parsed JSON across queries
// - CompiledPath for skipping path parsing overhead
// - SetCreate and SetMultipleCreate for path auto-creation
// - DeleteClean for automatic cleanup after deletion
//
// Run: go run -tags=example examples/17_advanced_patterns.go

func main() {
	fmt.Println("JSON Library - Advanced Patterns")
	fmt.Println("=================================")

	// 1. SAFEGET AND ACCESSRESULT
	demonstrateSafeGet()

	// 2. PRE-PARSE PATTERN
	demonstratePreParse()

	// 3. COMPILED PATHS
	demonstrateCompiledPaths()

	// 4. SETCREATE AND DELETECLEAN
	demonstrateCreateAndClean()

	// 5. PERFORMANCE COMPARISON
	demonstratePerformanceComparison()

	fmt.Println("\nAdvanced patterns examples complete!")
}

func demonstrateSafeGet() {
	fmt.Println("1. SafeGet and AccessResult")
	fmt.Println("-----------------------------")

	testJSON := `{
		"user": {
			"name": "Alice",
			"age": 30,
			"email": null
		},
		"settings": {
			"theme": "dark"
		}
	}`

	// SafeGet never returns an error - returns AccessResult instead
	result := json.SafeGet(testJSON, "user.name")
	fmt.Printf("   user.name: exists=%t, value=%v, type=%s\n", result.Exists, result.Value, result.Type)

	result = json.SafeGet(testJSON, "user.missing")
	fmt.Printf("   user.missing: exists=%t, value=%v\n", result.Exists, result.Value)

	result = json.SafeGet(testJSON, "user.email")
	fmt.Printf("   user.email: exists=%t, value=%v\n", result.Exists, result.Value)

	result = json.SafeGet(testJSON, "settings.theme")
	fmt.Printf("   settings.theme: exists=%t, value=%v\n", result.Exists, result.Value)

	// SafeGet with invalid path
	result = json.SafeGet(testJSON, "")
	fmt.Printf("   (empty path): exists=%t\n", result.Exists)

	// Processor-level SafeGet
	processor, _ := json.New(json.DefaultConfig()) // OK: preset config always valid
	defer processor.Close()

	result = processor.SafeGet(testJSON, "user.age")
	fmt.Printf("\n   Processor.SafeGet('user.age'): exists=%t, value=%v\n", result.Exists, result.Value)

	// AccessResult type conversion methods
	ageResult := json.SafeGet(testJSON, "user.age")
	if age, err := ageResult.AsInt(); err == nil {
		fmt.Printf("   AsInt(): %d\n", age)
	}
	if age, err := ageResult.AsFloat64(); err == nil {
		fmt.Printf("   AsFloat64(): %.1f\n", age)
	}

	nameResult := json.SafeGet(testJSON, "user.name")
	if name, err := nameResult.AsString(); err == nil {
		fmt.Printf("   AsString(): %s\n", name)
	}

	missingResult := json.SafeGet(testJSON, "user.missing")
	fmt.Printf("   missing.Ok(): %t, UnwrapOr('default'): %v\n",
		missingResult.Ok(), missingResult.UnwrapOr("default"))
}

func demonstratePreParse() {
	fmt.Println("\n2. PreParse Pattern (reuse parsed JSON)")
	fmt.Println("------------------------------------------")

	testJSON := `{
		"users": [
			{"id": 1, "name": "Alice", "score": 95},
			{"id": 2, "name": "Bob", "score": 82},
			{"id": 3, "name": "Charlie", "score": 78}
		],
		"metadata": {
			"total": 3,
			"page": 1
		}
	}`

	processor, _ := json.New(json.DefaultConfig()) // OK: preset config always valid
	defer processor.Close()

	// PreParse parses JSON once, then query multiple paths without re-parsing
	parsed, err := processor.PreParse(testJSON)
	if err != nil {
		fmt.Printf("   PreParse error: %v\n", err)
		return
	}
	fmt.Println("   PreParse: JSON parsed once")

	// Query multiple paths from pre-parsed data
	paths := []string{
		"users[0].name",
		"users[1].score",
		"metadata.total",
		"users[2].id",
	}

	fmt.Println("   Querying pre-parsed data:")
	for _, path := range paths {
		val, err := processor.GetFromParsed(parsed, path)
		if err != nil {
			fmt.Printf("   - %s: error: %v\n", path, err)
		} else {
			fmt.Printf("   - %s: %v\n", path, val)
		}
	}

	// SetFromParsed returns a new ParsedJSON for chaining
	newParsed, err := processor.SetFromParsed(parsed, "metadata.page", 2)
	if err != nil {
		fmt.Printf("   SetFromParsed error: %v\n", err)
		return
	}

	val, _ := processor.GetFromParsed(newParsed, "metadata.page")
	fmt.Printf("\n   After SetFromParsed('metadata.page', 2): %v\n", val)

	// Original parsed data is unchanged
	origVal, _ := processor.GetFromParsed(parsed, "metadata.page")
	fmt.Printf("   Original parsed still has: %v\n", origVal)
}

func demonstrateCompiledPaths() {
	fmt.Println("\n3. Compiled Paths (skip path parsing)")
	fmt.Println("---------------------------------------")

	processor, _ := json.New(json.DefaultConfig()) // OK: preset config always valid
	defer processor.Close()

	testJSON := `{"level1": {"level2": {"level3": {"value": "deep"}}}}`

	// Compile a path for repeated use
	compiled, err := processor.CompilePath("level1.level2.level3.value")
	if err != nil {
		fmt.Printf("   CompilePath error: %v\n", err)
		return
	}
	defer compiled.Release()
	fmt.Println("   Compiled path: level1.level2.level3.value")

	// Use compiled path for fast repeated access
	for i := 0; i < 5; i++ {
		val, err := processor.GetCompiled(testJSON, compiled)
		if err != nil {
			fmt.Printf("   GetCompiled error: %v\n", err)
			return
		}
		fmt.Printf("   GetCompiled iteration %d: %v\n", i+1, val)
	}
}

func demonstrateCreateAndClean() {
	fmt.Println("\n4. SetCreate and DeleteClean")
	fmt.Println("------------------------------")

	processor, _ := json.New(json.DefaultConfig()) // OK: preset config always valid
	defer processor.Close()

	testJSON := `{"user": {"name": "Alice"}}`

	// SetCreate automatically creates intermediate paths
	result, err := processor.SetCreate(testJSON, "user.profile.avatar", "alice.png")
	if err != nil {
		fmt.Printf("   SetCreate error: %v\n", err)
		return
	}
	avatar := json.GetString(result, "user.profile.avatar", "")
	fmt.Printf("   SetCreate('user.profile.avatar'): %s\n", avatar)

	// SetMultipleCreate for batch path creation
	updates := map[string]any{
		"user.profile.bio":     "Software Engineer",
		"user.profile.website": "https://alice.dev",
		"user.settings.theme":  "dark",
	}
	result2, err := processor.SetMultipleCreate(testJSON, updates)
	if err != nil {
		fmt.Printf("   SetMultipleCreate error: %v\n", err)
		return
	}
	fmt.Println("   SetMultipleCreate result:")
	fmt.Printf("   - bio: %s\n", json.GetString(result2, "user.profile.bio", ""))
	fmt.Printf("   - website: %s\n", json.GetString(result2, "user.profile.website", ""))
	fmt.Printf("   - theme: %s\n", json.GetString(result2, "user.settings.theme", ""))

	// DeleteClean removes path and cleans up nulls and empty arrays
	dataWithNulls := `{
		"user": {
			"name": "Bob",
			"temp": "value",
			"profile": {"avatar": "bob.png"}
		}
	}`

	cleaned, err := processor.DeleteClean(dataWithNulls, "user.temp")
	if err != nil {
		fmt.Printf("   DeleteClean error: %v\n", err)
		return
	}
	fmt.Printf("\n   DeleteClean('user.temp'): %s\n", cleaned)
}

func demonstratePerformanceComparison() {
	fmt.Println("\n5. Performance Comparison")
	fmt.Println("---------------------------")

	testJSON := `{"users": [{"id": 1, "name": "Alice"}, {"id": 2, "name": "Bob"}], "config": {"version": "1.0"}}`
	path := "users[0].name"
	iterations := 1000

	processor, _ := json.New(json.DefaultConfig()) // OK: preset config always valid
	defer processor.Close()

	// Regular Get
	start := time.Now()
	for i := 0; i < iterations; i++ {
		_, _ = processor.Get(testJSON, path)
	}
	regularDuration := time.Since(start)

	// CompiledPath
	compiled, _ := processor.CompilePath(path)
	start = time.Now()
	for i := 0; i < iterations; i++ {
		_, _ = processor.GetCompiled(testJSON, compiled)
	}
	compiledDuration := time.Since(start)

	// PreParse
	parsed, _ := processor.PreParse(testJSON)
	start = time.Now()
	for i := 0; i < iterations; i++ {
		_, _ = processor.GetFromParsed(parsed, path)
	}
	preparseDuration := time.Since(start)

	// SafeGet
	start = time.Now()
	for i := 0; i < iterations; i++ {
		_ = processor.SafeGet(testJSON, path)
	}
	safegetDuration := time.Since(start)

	fmt.Printf("   %d iterations, path=%s:\n", iterations, path)
	fmt.Printf("   Regular Get:     %v\n", regularDuration)
	fmt.Printf("   CompiledPath:    %v\n", compiledDuration)
	fmt.Printf("   PreParse:        %v\n", preparseDuration)
	fmt.Printf("   SafeGet:         %v\n", safegetDuration)

	// Show relative performance
	fmt.Printf("\n   Relative to Regular Get:\n")
	if compiledDuration > 0 {
		fmt.Printf("   CompiledPath:    %.2fx\n", float64(regularDuration)/float64(compiledDuration))
	}
	if preparseDuration > 0 {
		fmt.Printf("   PreParse:        %.2fx\n", float64(regularDuration)/float64(preparseDuration))
	}
}
