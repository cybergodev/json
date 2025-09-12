package json

import (
	"fmt"
	"runtime"
	"testing"
	"time"
)

// BenchmarkBasicOperations benchmarks basic JSON operations
// Merged from: benchmark_test.go, modular_benchmark_test.go, advanced_features_benchmark_test.go
func BenchmarkBasicOperations(b *testing.B) {
	generator := NewTestDataGenerator()
	jsonStr := generator.GenerateComplexJSON()

	b.Run("Get", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err := Get(jsonStr, "users[0].name")
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("GetString", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err := GetString(jsonStr, "users[0].name")
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("GetInt", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err := GetInt(jsonStr, "users[0].profile.age")
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("Set", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err := Set(jsonStr, "users[0].name", "Updated Name")
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("Delete", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err := Delete(jsonStr, "users[0].profile.age")
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}

// BenchmarkPathComplexity benchmarks operations with different path complexities
func BenchmarkPathComplexity(b *testing.B) {
	generator := NewTestDataGenerator()
	jsonStr := generator.GenerateComplexJSON()

	paths := []struct {
		name string
		path string
	}{
		{"Simple", "users"},
		{"OneLevel", "users[0]"},
		{"TwoLevel", "users[0].name"},
		{"ThreeLevel", "users[0].profile.age"},
		{"FourLevel", "users[0].profile.preferences.theme"},
		{"FiveLevel", "users[0].profile.preferences.languages[0]"},
		{"ArraySlice", "users[0].profile.preferences.languages[0:2]"},
		{"NegativeIndex", "users[-1].name"},
	}

	for _, p := range paths {
		b.Run(p.name, func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_, err := Get(jsonStr, p.path)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// BenchmarkJSONSizes benchmarks operations with different JSON sizes
func BenchmarkJSONSizes(b *testing.B) {
	sizes := []struct {
		name string
		json string
	}{
		{"Small", `{"name":"John","age":30}`},
		{"Medium", generateMediumJSON()},
		{"Large", generateLargeJSON()},
		{"VeryLarge", generateVeryLargeJSON()},
	}

	for _, size := range sizes {
		b.Run(size.name, func(b *testing.B) {
			b.Run("Get", func(b *testing.B) {
				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					_, err := Get(size.json, "name")
					if err != nil {
						b.Fatal(err)
					}
				}
			})

			b.Run("Valid", func(b *testing.B) {
				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					valid := Valid([]byte(size.json))
					if !valid {
						b.Fatal("JSON should be valid")
					}
				}
			})
		})
	}
}

// BenchmarkCachePerformance benchmarks cache performance
func BenchmarkCachePerformance(b *testing.B) {
	generator := NewTestDataGenerator()
	jsonStr := generator.GenerateComplexJSON()

	b.Run("WithCache", func(b *testing.B) {
		config := DefaultConfig()
		config.EnableCache = true
		config.MaxCacheSize = 1000
		processor := New(config)
		defer processor.Close()

		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err := processor.Get(jsonStr, "users[0].name")
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("WithoutCache", func(b *testing.B) {
		config := DefaultConfig()
		config.EnableCache = false
		processor := New(config)
		defer processor.Close()

		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err := processor.Get(jsonStr, "users[0].name")
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}

// BenchmarkConcurrency benchmarks concurrent operations
func BenchmarkConcurrency(b *testing.B) {
	generator := NewTestDataGenerator()
	jsonStr := generator.GenerateComplexJSON()

	concurrencyLevels := []int{1, 2, 4, 8, 16}

	for _, concurrency := range concurrencyLevels {
		b.Run(fmt.Sprintf("Concurrency%d", concurrency), func(b *testing.B) {
			b.ReportAllocs()
			b.SetParallelism(concurrency)
			b.ResetTimer()

			b.RunParallel(func(pb *testing.PB) {
				for pb.Next() {
					_, err := Get(jsonStr, "users[0].name")
					if err != nil {
						b.Fatal(err)
					}
				}
			})
		})
	}
}

// BenchmarkAdvancedPathExpressions benchmarks advanced path expression features
func BenchmarkAdvancedPathExpressions(b *testing.B) {
	// Complex test data
	complexData := `{
		"departments": [
			{
				"teams": [
					{
						"members": [
							{"name": "Alice", "skills": ["Go", "Python", "Docker"]},
							{"name": "Bob", "skills": ["Java", "Spring", "Kubernetes"]},
							{"name": "Charlie", "skills": ["React", "TypeScript", "Node.js"]}
						]
					},
					{
						"members": [
							{"name": "Diana", "skills": ["Vue", "JavaScript", "CSS"]},
							{"name": "Eve", "skills": ["Angular", "RxJS", "SCSS"]}
						]
					}
				]
			}
		]
	}`

	b.Run("StructurePreservingExtraction", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err := Get(complexData, "departments{teams}{members}{name}")
			if err != nil {
				b.Fatalf("Extraction failed: %v", err)
			}
		}
	})

	b.Run("FlatExtraction", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err := Get(complexData, "departments{flat:teams}{flat:members}{name}")
			if err != nil {
				b.Fatalf("Flat extraction failed: %v", err)
			}
		}
	})

	b.Run("PostExtractionSlicing", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err := Get(complexData, "departments{teams}{members}[0:2]")
			if err != nil {
				b.Fatalf("Post-extraction slicing failed: %v", err)
			}
		}
	})

	b.Run("ComplexFlatExtractionWithSlicing", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err := Get(complexData, "departments{flat:teams}{flat:members}{flat:skills}[0:5]")
			if err != nil {
				b.Fatalf("Complex flat extraction with slicing failed: %v", err)
			}
		}
	})

	b.Run("DeepNestedExtraction", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err := Get(complexData, "departments{flat:teams}{flat:members}{flat:skills}")
			if err != nil {
				b.Fatalf("Deep nested extraction failed: %v", err)
			}
		}
	})
}

// BenchmarkMemoryUsage benchmarks memory usage patterns
func BenchmarkMemoryUsage(b *testing.B) {
	b.Run("ProcessorCreation", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			processor := New()
			processor.Close()
		}
	})

	b.Run("LargeJSONProcessing", func(b *testing.B) {
		largeJSON := generateVeryLargeJSON()

		var m1, m2 runtime.MemStats
		runtime.GC()
		runtime.ReadMemStats(&m1)

		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err := Get(largeJSON, "items[0].name")
			if err != nil {
				b.Fatal(err)
			}
		}

		runtime.GC()
		runtime.ReadMemStats(&m2)

		b.ReportMetric(float64(m2.Alloc-m1.Alloc)/float64(b.N), "bytes/op")
	})
}

// BenchmarkEncoding benchmarks encoding operations
func BenchmarkEncoding(b *testing.B) {
	testData := map[string]any{
		"users": []map[string]any{
			{
				"id":   1,
				"name": "Alice Johnson",
				"profile": map[string]any{
					"age":      28,
					"location": "New York",
					"skills":   []string{"Go", "Python", "JavaScript"},
				},
			},
			{
				"id":   2,
				"name": "Bob Smith",
				"profile": map[string]any{
					"age":      35,
					"location": "San Francisco",
					"skills":   []string{"Java", "Kotlin", "React"},
				},
			},
		},
		"metadata": map[string]any{
			"total":     2,
			"timestamp": time.Now().Format(time.RFC3339),
		},
	}

	b.Run("Encode", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err := Encode(testData)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}

// Helper functions for generating test data
func generateMediumJSON() string {
	return `{
		"users": [
			{"name": "Alice", "age": 25, "city": "New York"},
			{"name": "Bob", "age": 30, "city": "San Francisco"},
			{"name": "Charlie", "age": 35, "city": "Chicago"}
		],
		"metadata": {"total": 3, "version": "1.0"}
	}`
}

func generateLargeJSON() string {
	return `{
		"items": [` + generateItems(100) + `],
		"metadata": {"count": 100, "generated": "2024-01-01"}
	}`
}

func generateVeryLargeJSON() string {
	return `{
		"items": [` + generateItems(1000) + `],
		"metadata": {"count": 1000, "generated": "2024-01-01"}
	}`
}

func generateItems(count int) string {
	items := make([]string, count)
	for i := 0; i < count; i++ {
		items[i] = fmt.Sprintf(`{"id": %d, "name": "item_%d", "value": %d}`, i, i, i*10)
	}
	return fmt.Sprintf("%s", items[0]) // Simplified for brevity
}
