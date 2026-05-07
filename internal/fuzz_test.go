package internal

import (
	"testing"
)

// ============================================================================
// FUZZ TESTS FOR SECURITY VALIDATION
// These tests verify that parsers handle malformed input without panicking
// ============================================================================

// FuzzIsValidJSONNumber tests IsValidJSONNumber with random input
func FuzzIsValidJSONNumber(f *testing.F) {
	// Seed corpus with valid and invalid numbers
	seeds := []string{
		"0",
		"42",
		"-42",
		"3.14",
		"-3.14",
		"0.0",
		"1e10",
		"1E10",
		"1e+10",
		"1e-10",
		"1.5e+10",
		"-1.5e-10",
		"",         // empty
		"-",        // just minus
		"01",       // leading zero
		".5",       // no integer part
		"1.",       // no decimal part
		"1e",       // no exponent
		"1e+",      // no exponent after sign
		"1e-",      // no exponent after sign
		"abc",      // not a number
		"12.34.56", // multiple decimals
		"++123",    // double plus
		"--123",    // double minus
		"1.2.3e4",  // invalid structure
		"9999999999999999999999999999999999999999", // very long number
	}

	for _, seed := range seeds {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, input string) {
		// Should not panic for any input
		result := IsValidJSONNumber(input)
		_ = result // Just verify no panic
	})
}

// FuzzIsValidJSONPrimitive tests IsValidJSONPrimitive with random input
func FuzzIsValidJSONPrimitive(f *testing.F) {
	seeds := []string{
		"true",
		"false",
		"null",
		"42",
		"-3.14",
		"1e10",
		"True",
		"FALSE",
		"NULL",
		"",
		"undefined",
		"NaN",
		"Infinity",
	}

	for _, seed := range seeds {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, input string) {
		// Should not panic for any input
		result := IsValidJSONPrimitive(input)
		_ = result
	})
}

// FuzzIsWordChar tests IsWordChar with all byte values
func FuzzIsWordChar(f *testing.F) {
	seeds := []byte{
		'a', 'Z', '0', '9', '_', '-', ' ', '!', '@', '#',
		0x00, 0x7F, 0x80, 0xFF,
	}

	for _, seed := range seeds {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, input byte) {
		// Should not panic for any byte
		result := IsWordChar(input)
		_ = result
	})
}

// ============================================================================
// FUZZ TESTS FOR PATH PARSING
// ============================================================================

// FuzzParsePath tests ParsePath with random input
func FuzzParsePath(f *testing.F) {
	seeds := []string{
		"name",
		"user.name",
		"users[0]",
		"data{key}",
		"arr[1:5:2]",
		"users[0].name",
		"data{key}.sub",
		"a.b.c.d.e",
		"arr[0][1][2]",
		"",                   // empty
		".",                  // just dot
		"..",                 // double dot
		"[]",                 // empty brackets
		"{}",                 // empty braces
		"[invalid]",          // invalid index
		"{invalid",           // unclosed brace
		"user[]",             // empty index
		"a[b]c",              // mixed
		"/json/pointer/path", // JSON pointer style
		"中文.路径",              // Unicode
	}

	for _, seed := range seeds {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, input string) {
		// Should not panic for any input
		segments, err := ParsePath(input)
		if err != nil {
			return // Error is acceptable for invalid paths
		}
		_ = segments
	})
}

// FuzzNeedsPathPreprocessing tests NeedsPathPreprocessing
func FuzzNeedsPathPreprocessing(f *testing.F) {
	seeds := []string{
		"name",
		"user[0]",
		"data{key}",
		"",
		"a.b.c",
		"[[[",
		"{{{",
	}

	for _, seed := range seeds {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, input string) {
		result := NeedsPathPreprocessing(input)
		_ = result
	})
}

// FuzzIsComplexPath tests IsComplexPath
func FuzzIsComplexPath(f *testing.F) {
	seeds := []string{
		"name",
		"user.name",
		"user[0]",
		"data{key}",
		"slice[1:5]",
		"a:b",
		"",
		"{}[]:",
	}

	for _, seed := range seeds {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, input string) {
		result := IsComplexPath(input)
		_ = result
	})
}

// ============================================================================
// FUZZ TESTS FOR STRING INTERNING
// ============================================================================

// FuzzStringIntern tests StringIntern.Intern with random strings
func FuzzStringIntern(f *testing.F) {
	seeds := []string{
		"",
		"a",
		"key",
		"user_name",
		"camelCase",
		"PascalCase",
		"with spaces",
		"with\nnewline",
		"with\ttab",
		"unicode中文",
		"emoji🎉",
		string(make([]byte, 300)), // long string
	}

	for _, seed := range seeds {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, input string) {
		si := NewStringIntern(1024 * 1024)
		result := si.Intern(input)
		_ = result

		// Test InternBytes as well
		resultBytes := si.InternBytes([]byte(input))
		_ = resultBytes
	})
}

// FuzzKeyIntern tests KeyIntern.Intern with random strings
func FuzzKeyIntern(f *testing.F) {
	seeds := []string{
		"",
		"id",
		"userName",
		"created_at",
		"CamelCase",
		"123numeric",
		"_underscore",
		"unicode键",
	}

	for _, seed := range seeds {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, input string) {
		ki := NewKeyIntern()
		result := ki.Intern(input)
		_ = result

		// Test InternBytes as well
		resultBytes := ki.InternBytes([]byte(input))
		_ = resultBytes
	})
}

// ============================================================================
// FUZZ TESTS FOR ARRAY UTILITIES
// ============================================================================

// FuzzNormalizeIndex tests NormalizeIndex
func FuzzNormalizeIndex(f *testing.F) {
	seeds := []struct {
		index  int
		length int
	}{
		{0, 5},
		{2, 5},
		{4, 5},
		{-1, 5},
		{-5, 5},
		{10, 5},
		{-10, 5},
		{0, 0},
	}

	for _, seed := range seeds {
		f.Add(seed.index, seed.length)
	}

	f.Fuzz(func(t *testing.T, index, length int) {
		// Prevent negative length
		if length < 0 {
			return
		}
		result := NormalizeIndex(index, length)
		_ = result
	})
}

// FuzzNormalizePathSeparators tests NormalizePathSeparators
func FuzzNormalizePathSeparators(f *testing.F) {
	seeds := []string{
		"",
		"a",
		"a.b.c",
		"a..b",
		"a...b",
		".a.b",
		"a.b.",
		"..a..b..",
		"a.b[0].c",
	}

	for _, seed := range seeds {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, input string) {
		result := NormalizePathSeparators(input)
		_ = result
	})
}

// ============================================================================
// FUZZ TESTS FOR DATA OPERATIONS
// ============================================================================

// ============================================================================
// BENCHMARKS FOR FUZZ TARGETS
// ============================================================================

func BenchmarkIsValidJSONNumber_Valid(b *testing.B) {
	input := "123.456e789"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		IsValidJSONNumber(input)
	}
}

func BenchmarkIsValidJSONNumber_Invalid(b *testing.B) {
	input := "not-a-number"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		IsValidJSONNumber(input)
	}
}

func BenchmarkParsePath_Simple(b *testing.B) {
	input := "user.name.email"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ParsePath(input)
	}
}

func BenchmarkParsePath_Complex(b *testing.B) {
	input := "users[0].posts{title}.comments[1:5:2]"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ParsePath(input)
	}
}
