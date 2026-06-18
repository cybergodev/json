package internal

import (
	"testing"
)

// TestHashUint64 tests the HashUint64 function
func TestHashUint64(t *testing.T) {
	tests := []struct {
		name string
		h    uint64
		v    uint64
	}{
		{"zero values", 0, 0},
		{"zero hash value", FNVOffsetBasis, 0},
		{"non-zero values", FNVOffsetBasis, 12345},
		{"max uint64", FNVOffsetBasis, ^uint64(0)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := HashUint64(tt.h, tt.v)
			// Verify deterministic behavior
			result2 := HashUint64(tt.h, tt.v)
			if result != result2 {
				t.Errorf("HashUint64 is not deterministic: %d != %d", result, result2)
			}
			// Different values should produce different results
			if tt.v != 0 {
				result3 := HashUint64(tt.h, tt.v+1)
				if result == result3 {
					t.Errorf("HashUint64 collision for %d and %d", tt.v, tt.v+1)
				}
			}
		})
	}
}

// TestHashBool tests the HashBool function
func TestHashBool(t *testing.T) {
	h := FNVOffsetBasis

	resultTrue := HashBool(h, true)
	resultFalse := HashBool(h, false)

	// True and false should produce different hashes
	if resultTrue == resultFalse {
		t.Errorf("HashBool(true) == HashBool(false): both = %d", resultTrue)
	}

	// Verify the actual values
	// true: h ^= 1, then *= FNVPrime
	expectedTrue := (h ^ 1) * FNVPrime
	if resultTrue != expectedTrue {
		t.Errorf("HashBool(true) = %d, want %d", resultTrue, expectedTrue)
	}

	// false: h ^= 0xFF, then *= FNVPrime
	expectedFalse := (h ^ 0xFF) * FNVPrime
	if resultFalse != expectedFalse {
		t.Errorf("HashBool(false) = %d, want %d", resultFalse, expectedFalse)
	}
}

// TestHashInt tests the HashInt function
func TestHashInt(t *testing.T) {
	tests := []struct {
		name string
		h    uint64
		v    int
	}{
		{"zero", FNVOffsetBasis, 0},
		{"positive", FNVOffsetBasis, 42},
		{"negative", FNVOffsetBasis, -42},
		{"large positive", FNVOffsetBasis, 1000000},
		{"large negative", FNVOffsetBasis, -1000000},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := HashInt(tt.h, tt.v)
			// HashInt should delegate to HashUint64
			expected := HashUint64(tt.h, uint64(tt.v))
			if result != expected {
				t.Errorf("HashInt(%d) = %d, want %d", tt.v, result, expected)
			}
		})
	}
}

// TestHashInt64 tests the HashInt64 function
func TestHashInt64(t *testing.T) {
	tests := []struct {
		name string
		h    uint64
		v    int64
	}{
		{"zero", FNVOffsetBasis, 0},
		{"positive", FNVOffsetBasis, 42},
		{"negative", FNVOffsetBasis, -42},
		{"max int64", FNVOffsetBasis, int64(1<<63 - 1)},
		{"min int64", FNVOffsetBasis, int64(-1 << 63)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := HashInt64(tt.h, tt.v)
			// HashInt64 should delegate to HashUint64
			expected := HashUint64(tt.h, uint64(tt.v))
			if result != expected {
				t.Errorf("HashInt64(%d) = %d, want %d", tt.v, result, expected)
			}
		})
	}
}

// TestHashString tests the HashString function
func TestHashString(t *testing.T) {
	tests := []struct {
		name string
		h    uint64
		s    string
	}{
		{"empty string", FNVOffsetBasis, ""},
		{"single char", FNVOffsetBasis, "a"},
		{"short string", FNVOffsetBasis, "hello"},
		{"long string", FNVOffsetBasis, "this is a longer string for testing"},
		{"unicode", FNVOffsetBasis, "hello world"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := HashString(tt.h, tt.s)
			// Verify deterministic behavior
			result2 := HashString(tt.h, tt.s)
			if result != result2 {
				t.Errorf("HashString is not deterministic: %d != %d", result, result2)
			}
		})
	}

	// Different strings should produce different results
	h := FNVOffsetBasis
	if HashString(h, "abc") == HashString(h, "def") {
		t.Error("HashString collision for different strings")
	}
}

// TestHashStringFNV1a tests the HashStringFNV1a function
func TestHashStringFNV1a(t *testing.T) {
	tests := []struct {
		name string
		s    string
	}{
		{"empty string", ""},
		{"single char", "a"},
		{"hello world", "hello world"},
		{"numbers", "12345"},
		{"special chars", "!@#$%^&*()"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := HashStringFNV1a(tt.s)
			// Verify deterministic behavior
			result2 := HashStringFNV1a(tt.s)
			if result != result2 {
				t.Errorf("HashStringFNV1a is not deterministic")
			}
		})
	}
}

// TestHashStringFNV1aSampled tests the sampled hash function
func TestHashStringFNV1aSampled(t *testing.T) {
	tests := []struct {
		name string
		s    string
	}{
		{"empty string", ""},
		{"small string", "hello"},
		{"medium string", makeString(2048)},
		{"large string", makeString(8192)},
		{"very large string", makeString(65536)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := HashStringFNV1aSampled(tt.s)
			// Verify deterministic behavior
			result2 := HashStringFNV1aSampled(tt.s)
			if result != result2 {
				t.Errorf("HashStringFNV1aSampled is not deterministic")
			}
		})
	}
}

// TestHashBytesFNV1a tests the HashBytesFNV1a function
func TestHashBytesFNV1a(t *testing.T) {
	tests := []struct {
		name string
		b    []byte
	}{
		{"empty", []byte{}},
		{"single byte", []byte{0x41}},
		{"multiple bytes", []byte("hello")},
		{"binary data", []byte{0x00, 0x01, 0x02, 0xFF, 0xFE}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := HashBytesFNV1a(tt.b)
			// Verify deterministic behavior
			result2 := HashBytesFNV1a(tt.b)
			if result != result2 {
				t.Errorf("HashBytesFNV1a is not deterministic")
			}
		})
	}
}

// TestHashConstants verifies FNV constants
func TestHashConstants(t *testing.T) {
	if FNVOffsetBasis != 14695981039346656037 {
		t.Errorf("FNVOffsetBasis = %d, want 14695981039346656037", FNVOffsetBasis)
	}
	if FNVPrime != 1099511628211 {
		t.Errorf("FNVPrime = %d, want 1099511628211", FNVPrime)
	}
	if LargeStringHashThreshold != 4096 {
		t.Errorf("LargeStringHashThreshold = %d, want 4096", LargeStringHashThreshold)
	}
}

// Benchmark tests
func BenchmarkHashUint64(b *testing.B) {
	h := FNVOffsetBasis
	for i := 0; i < b.N; i++ {
		h = HashUint64(h, uint64(i))
	}
}

func BenchmarkHashBool(b *testing.B) {
	h := FNVOffsetBasis
	for i := 0; i < b.N; i++ {
		h = HashBool(h, i%2 == 0)
	}
}

func BenchmarkHashString(b *testing.B) {
	s := "test string for benchmarking"
	h := FNVOffsetBasis
	for i := 0; i < b.N; i++ {
		h = HashString(h, s)
	}
}

func BenchmarkHashStringFNV1a(b *testing.B) {
	s := "test string for benchmarking"
	for i := 0; i < b.N; i++ {
		_ = HashStringFNV1a(s)
	}
}

func BenchmarkHashStringFNV1aSampled_Small(b *testing.B) {
	s := "small string"
	for i := 0; i < b.N; i++ {
		_ = HashStringFNV1aSampled(s)
	}
}

func BenchmarkHashStringFNV1aSampled_Large(b *testing.B) {
	s := makeString(8192)
	for i := 0; i < b.N; i++ {
		_ = HashStringFNV1aSampled(s)
	}
}

// Helper function to create a string of given size
func makeString(size int) string {
	result := make([]byte, size)
	for i := 0; i < size; i++ {
		result[i] = byte('a' + (i % 26))
	}
	return string(result)
}
