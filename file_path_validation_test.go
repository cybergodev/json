package json

import (
	"runtime"
	"testing"
)

// TestFilePathValidationImprovements tests the improved file path validation logic
func TestFilePathValidationImprovements(t *testing.T) {
	processor := New()
	defer processor.Close()

	t.Run("RelativePathsAllowed", func(t *testing.T) {
		// These safe relative paths should be allowed
		validRelativePaths := []string{
			"dev_test/file.json",
			"config/production.json",
			"data/users.json",
			"test.json",
			"folder/subfolder/deep/file.json",
		}

		for _, path := range validRelativePaths {
			err := processor.validateFilePath(path)
			if err != nil {
				t.Errorf("Valid relative path '%s' was rejected: %v", path, err)
			}
		}
	})

	t.Run("AbsoluteSystemPathsBlocked", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("Skipping Unix system path tests on Windows")
		}

		// These absolute system paths should be blocked
		invalidSystemPaths := []string{
			"/dev/null",
			"/proc/self/environ",
			"/sys/kernel/debug",
			"/etc/passwd",
			"/etc/shadow",
		}

		for _, path := range invalidSystemPaths {
			err := processor.validateFilePath(path)
			if err == nil {
				t.Errorf("Invalid system path '%s' was allowed", path)
			}
		}
	})

	t.Run("WindowsDeviceNamesCorrectlyDetected", func(t *testing.T) {
		if runtime.GOOS != "windows" {
			t.Skip("Skipping Windows device name tests on non-Windows platform")
		}

		// These should be blocked (exact device names)
		invalidDeviceNames := []string{
			"CON",
			"CON.txt",
			"PRN",
			"PRN.log",
			"AUX",
			"NUL",
			"COM1",
			"COM1.dat",
			"LPT1",
			"LPT9.txt",
		}

		for _, path := range invalidDeviceNames {
			err := processor.validateFilePath(path)
			if err == nil {
				t.Errorf("Windows device name '%s' was allowed", path)
			}
		}

		// These should be allowed (not exact device names)
		validFilenames := []string{
			"config.json",
			"console.txt",
			"mycon.json",
			"printer.log",
			"auxiliary.dat",
			"com10.txt", // COM10 is not reserved
			"lpt0.txt",  // LPT0 is not reserved
			"connection.json",
		}

		for _, path := range validFilenames {
			err := processor.validateFilePath(path)
			if err != nil {
				t.Errorf("Valid filename '%s' was rejected: %v", path, err)
			}
		}
	})

	t.Run("PathTraversalStillBlocked", func(t *testing.T) {
		// Path traversal attempts should still be blocked for security
		pathTraversalAttempts := []string{
			"../parent/file.json",       // Parent directory access
			"../../etc/passwd",          // Multiple level traversal
			"..\\..\\windows\\system32", // Windows path traversal
			"./../../sensitive.txt",     // Mixed traversal
		}

		for _, path := range pathTraversalAttempts {
			err := processor.validateFilePath(path)
			if err == nil {
				t.Errorf("Path traversal attempt '%s' was allowed", path)
			}
		}
	})

	t.Run("NullBytesBlocked", func(t *testing.T) {
		// Null bytes should be blocked
		pathsWithNullBytes := []string{
			"file\x00.json",
			"path/to\x00/file.json",
		}

		for _, path := range pathsWithNullBytes {
			err := processor.validateFilePath(path)
			if err == nil {
				t.Errorf("Path with null byte '%s' was allowed", path)
			}
		}
	})

	t.Run("ExcessivelyLongPathsBlocked", func(t *testing.T) {
		// Very long paths should be blocked
		longPath := string(make([]byte, 5000))
		err := processor.validateFilePath(longPath)
		if err == nil {
			t.Error("Excessively long path was allowed")
		}
	})

	t.Run("EmptyPathBlocked", func(t *testing.T) {
		err := processor.validateFilePath("")
		if err == nil {
			t.Error("Empty path was allowed")
		}
	})
}

// TestCacheSizeEstimationPerformance tests the improved cache size estimation
func TestCacheSizeEstimationPerformance(t *testing.T) {
	t.Run("LargeArrayEstimation", func(t *testing.T) {
		// Create a large array
		largeArray := make([]any, 10000)
		for i := range largeArray {
			largeArray[i] = i
		}

		// This should complete quickly without stack overflow
		size := estimateSize(largeArray)
		if size <= 0 {
			t.Error("Invalid size estimation for large array")
		}
		t.Logf("Large array (10000 elements) estimated size: %d bytes", size)
	})

	t.Run("LargeMapEstimation", func(t *testing.T) {
		// Create a large map
		largeMap := make(map[string]any, 10000)
		for i := 0; i < 10000; i++ {
			largeMap[string(rune('a'+i%26))+string(rune('0'+i%10))] = i
		}

		// This should complete quickly without stack overflow
		size := estimateSize(largeMap)
		if size <= 0 {
			t.Error("Invalid size estimation for large map")
		}
		t.Logf("Large map (10000 entries) estimated size: %d bytes", size)
	})

	t.Run("NestedStructureEstimation", func(t *testing.T) {
		// Create a nested structure
		nested := map[string]any{
			"level1": map[string]any{
				"level2": map[string]any{
					"level3": []any{1, 2, 3, 4, 5},
				},
			},
		}

		size := estimateSize(nested)
		if size <= 0 {
			t.Error("Invalid size estimation for nested structure")
		}
		t.Logf("Nested structure estimated size: %d bytes", size)
	})

	t.Run("EmptyCollections", func(t *testing.T) {
		emptyArray := []any{}
		emptyMap := map[string]any{}

		arraySize := estimateSize(emptyArray)
		mapSize := estimateSize(emptyMap)

		if arraySize != 24 { // slice header size
			t.Errorf("Empty array size should be 24, got %d", arraySize)
		}
		if mapSize != 48 { // map header size
			t.Errorf("Empty map size should be 48, got %d", mapSize)
		}
	})
}

// BenchmarkCacheSizeEstimation benchmarks the cache size estimation performance
func BenchmarkCacheSizeEstimation(b *testing.B) {
	b.Run("SmallArray", func(b *testing.B) {
		smallArray := []any{1, 2, 3, 4, 5}
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = estimateSize(smallArray)
		}
	})

	b.Run("LargeArray", func(b *testing.B) {
		largeArray := make([]any, 1000)
		for i := range largeArray {
			largeArray[i] = i
		}
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = estimateSize(largeArray)
		}
	})

	b.Run("SmallMap", func(b *testing.B) {
		smallMap := map[string]any{
			"key1": "value1",
			"key2": "value2",
			"key3": "value3",
		}
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = estimateSize(smallMap)
		}
	})

	b.Run("LargeMap", func(b *testing.B) {
		largeMap := make(map[string]any, 1000)
		for i := 0; i < 1000; i++ {
			largeMap[string(rune('a'+i%26))+string(rune('0'+i%10))] = i
		}
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = estimateSize(largeMap)
		}
	})
}
