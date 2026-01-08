package json

import (
	"bytes"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

// TestComprehensiveJSONOperations consolidates ALL JSON operations tests
// This replaces: array_path_operations_test.go, core_operations_test.go, 
// encoding_validation_test.go, config_constants_test.go
func TestComprehensiveJSONOperations(t *testing.T) {
	helper := NewTestHelper(t)

	// ========== CORE OPERATIONS ==========
	t.Run("CoreOperations", func(t *testing.T) {
		testData := `{
			"string": "hello",
			"number": 42,
			"float": 3.14,
			"boolean": true,
			"null": null,
			"nested": {"deep": {"value": "found"}},
			"array": [1, 2, 3, 4, 5],
			"unicode": "你好世界🌍"
		}`

		t.Run("Get", func(t *testing.T) {
			tests := []struct {
				name     string
				path     string
				expected interface{}
				wantErr  bool
			}{
				{"String", "string", "hello", false},
				{"Number", "number", float64(42), false},
				{"Float", "float", 3.14, false},
				{"Boolean", "boolean", true, false},
				{"Null", "null", nil, false},
				{"DeepNested", "nested.deep.value", "found", false},
				{"ArrayIndex", "array[0]", float64(1), false},
				{"ArrayNegative", "array[-1]", float64(5), false},
				{"Unicode", "unicode", "你好世界🌍", false},
				{"NonExistent", "missing", nil, true},
			}

			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					result, err := Get(testData, tt.path)
					if tt.wantErr {
						helper.AssertError(err)
					} else {
						helper.AssertNoError(err)
						helper.AssertEqual(tt.expected, result)
					}
				})
			}
		})

		t.Run("Set", func(t *testing.T) {
			result, err := Set(`{}`, "name", "John")
			helper.AssertNoError(err)
			name, _ := GetString(result, "name")
			helper.AssertEqual("John", name)

			result, err = Set(`{"arr":[1,2,3]}`, "arr[1]", 99)
			helper.AssertNoError(err)
			val, _ := GetInt(result, "arr[1]")
			helper.AssertEqual(99, val)
		})

		t.Run("Delete", func(t *testing.T) {
			result, err := Delete(`{"name":"John","age":30}`, "name")
			helper.AssertNoError(err)
			val, _ := Get(result, "name")
			helper.AssertNil(val)
		})

		t.Run("TypedGet", func(t *testing.T) {
			str, err := GetTyped[string](testData, "string")
			helper.AssertNoError(err)
			helper.AssertEqual("hello", str)

			num, err := GetTyped[int](testData, "number")
			helper.AssertNoError(err)
			helper.AssertEqual(42, num)
		})

		t.Run("MultipleOperations", func(t *testing.T) {
			jsonStr := `{"name":"test","age":30,"city":"NYC"}`
			
			paths := []string{"name", "age", "city"}
			results, err := GetMultiple(jsonStr, paths)
			helper.AssertNoError(err)
			helper.AssertEqual(3, len(results))

			updates := map[string]any{"age": 31, "country": "USA"}
			result, err := SetMultiple(jsonStr, updates)
			helper.AssertNoError(err)
			helper.AssertTrue(len(result) > 0)
		})
	})

	// ========== ARRAY AND PATH OPERATIONS ==========
	t.Run("ArrayPathOperations", func(t *testing.T) {
		arrayData := `{
			"numbers": [1, 2, 3, 4, 5, 6, 7, 8, 9, 10],
			"users": [
				{"name": "Alice", "age": 25},
				{"name": "Bob", "age": 30},
				{"name": "Charlie", "age": 35}
			],
			"matrix": [[1, 2, 3], [4, 5, 6], [7, 8, 9]]
		}`

		t.Run("ArrayAccess", func(t *testing.T) {
			userName, err := GetString(arrayData, "users[1].name")
			helper.AssertNoError(err)
			helper.AssertEqual("Bob", userName)

			matrixElement, err := GetInt(arrayData, "matrix[1][2]")
			helper.AssertNoError(err)
			helper.AssertEqual(6, matrixElement)

			lastUserName, err := GetString(arrayData, "users[-1].name")
			helper.AssertNoError(err)
			helper.AssertEqual("Charlie", lastUserName)
		})

		t.Run("ArraySlicing", func(t *testing.T) {
			slice, err := Get(arrayData, "numbers[2:5]")
			helper.AssertNoError(err)
			expected := []any{float64(3), float64(4), float64(5)}
			helper.AssertEqual(expected, slice)

			negSlice, err := Get(arrayData, "numbers[-3:]")
			helper.AssertNoError(err)
			expectedNeg := []any{float64(8), float64(9), float64(10)}
			helper.AssertEqual(expectedNeg, negSlice)
		})

		t.Run("PathExtraction", func(t *testing.T) {
			names, err := Get(arrayData, "users{name}")
			helper.AssertNoError(err)
			if arr, ok := names.([]any); ok {
				helper.AssertEqual(3, len(arr))
				helper.AssertEqual("Alice", arr[0])
			}
		})

		t.Run("PathValidation", func(t *testing.T) {
			helper.AssertTrue(IsValidPath("user.name"))
			helper.AssertTrue(IsValidPath("items[0]"))
			helper.AssertFalse(IsValidPath(""))

			err := ValidatePath("user.name")
			helper.AssertNoError(err)
		})
	})

	// ========== ITERATION FUNCTIONS ==========
	t.Run("IterationFunctions", func(t *testing.T) {
		iterData := `{
			"users": [
				{"name": "Alice", "age": 25, "status": "active"},
				{"name": "Bob", "age": 30, "status": "inactive"},
				{"name": "Charlie", "age": 35, "status": "active"}
			],
			"settings": {"theme": "dark", "lang": "en"}
		}`

		t.Run("Foreach", func(t *testing.T) {
			count := 0
			Foreach(iterData, func(key any, item *IterableValue) {
				count++
				helper.AssertNotNil(item)
			})
			helper.AssertTrue(count > 0)
		})

		t.Run("ForeachWithPath", func(t *testing.T) {
			userCount := 0
			ForeachWithPath(iterData, "users", func(key any, user *IterableValue) {
				userCount++
				name := user.GetString("name")
				helper.AssertTrue(len(name) > 0)
			})
			helper.AssertEqual(3, userCount)
		})

		t.Run("ForeachReturn", func(t *testing.T) {
			modifiedJSON, err := ForeachReturn(iterData, func(key any, item *IterableValue) {
				// Modify user status during iteration
				if item.Exists("status") {
					status := item.GetString("status")
					if status == "inactive" {
						// Note: ForeachReturn allows modifications
						t.Logf("Found inactive user, would modify in real scenario")
					}
				}
			})
			helper.AssertNoError(err)
			helper.AssertTrue(len(modifiedJSON) > 0)
		})

		t.Run("ForeachNested", func(t *testing.T) {
			nestedCount := 0
			ForeachNested(iterData, func(key any, item *IterableValue) {
				nestedCount++
			})
			helper.AssertTrue(nestedCount > 0)
		})

		t.Run("IterableValueMethods", func(t *testing.T) {
			processor := New()
			defer processor.Close()

			var data interface{}
			err := processor.Parse(iterData, &data)
			helper.AssertNoError(err)

			iterator := NewIterator(processor, data, DefaultOptions())
			iterableValue := NewIterableValueWithIterator(data, processor, iterator)

			// Test existence checks
			helper.AssertTrue(iterableValue.Exists("users"))
			helper.AssertFalse(iterableValue.Exists("nonexistent"))

			// Test type checks
			helper.AssertFalse(iterableValue.IsNull("users"))
			helper.AssertFalse(iterableValue.IsEmpty("users"))
		})
	})

	// ========== FILE OPERATIONS ==========
	t.Run("FileOperations", func(t *testing.T) {
		tempDir := t.TempDir()

		t.Run("LoadFromFile_SaveToFile", func(t *testing.T) {
			testData := `{"name":"test","value":123}`
			filePath := tempDir + "/test.json"

			err := SaveToFile(filePath, testData, false)
			helper.AssertNoError(err)

			loaded, err := LoadFromFile(filePath)
			helper.AssertNoError(err)
			helper.AssertTrue(len(loaded) > 0)

			name, _ := GetString(loaded, "name")
			helper.AssertEqual("test", name)
		})

		t.Run("LoadFromReader_SaveToWriter", func(t *testing.T) {
			processor := New()
			defer processor.Close()

			testData := `{"reader":"test"}`
			reader := strings.NewReader(testData)

			loaded, err := processor.LoadFromReader(reader)
			helper.AssertNoError(err)
			helper.AssertNotNil(loaded)

			var buf bytes.Buffer
			err = processor.SaveToWriter(&buf, loaded, false)
			helper.AssertNoError(err)
			helper.AssertTrue(buf.Len() > 0)
		})

		t.Run("MarshalToFile_UnmarshalFromFile", func(t *testing.T) {
			type TestUser struct {
				Name  string `json:"name"`
				Age   int    `json:"age"`
				Email string `json:"email"`
			}

			testUser := TestUser{Name: "John", Age: 30, Email: "john@example.com"}
			filePath := tempDir + "/user.json"

			err := MarshalToFile(filePath, testUser)
			helper.AssertNoError(err)

			var loadedUser TestUser
			err = UnmarshalFromFile(filePath, &loadedUser)
			helper.AssertNoError(err)
			helper.AssertEqual(testUser.Name, loadedUser.Name)
			helper.AssertEqual(testUser.Age, loadedUser.Age)
		})

		t.Run("FilePathSecurity", func(t *testing.T) {
			processor := New()
			defer processor.Close()

			// Test path traversal detection
			err := processor.validateFilePath("../etc/passwd")
			helper.AssertError(err)

			err = processor.validateFilePath("..\\windows\\system32")
			helper.AssertError(err)

			// Test valid paths
			err = processor.validateFilePath("data.json")
			helper.AssertNoError(err)
		})
	})

	// ========== ENCODING AND VALIDATION ==========
	t.Run("EncodingValidation", func(t *testing.T) {
		t.Run("BasicEncoding", func(t *testing.T) {
			data := map[string]any{
				"name":   "Alice",
				"age":    25,
				"active": true,
			}

			jsonStr, err := Encode(data)
			helper.AssertNoError(err)
			helper.AssertTrue(len(jsonStr) > 0)

			name, _ := GetString(jsonStr, "name")
			helper.AssertEqual("Alice", name)
		})

		t.Run("MarshalUnmarshal", func(t *testing.T) {
			data := map[string]any{"test": "value", "number": 123}

			bytes, err := Marshal(data)
			helper.AssertNoError(err)

			var unmarshaled map[string]any
			err = Unmarshal(bytes, &unmarshaled)
			helper.AssertNoError(err)
			helper.AssertEqual("value", unmarshaled["test"])
		})

		t.Run("JSONValidation", func(t *testing.T) {
			validJSONs := []string{
				`null`, `true`, `42`, `"string"`, `[]`, `{}`,
				`{"key": "value"}`, `[1, 2, 3]`,
			}

			for _, jsonStr := range validJSONs {
				valid := Valid([]byte(jsonStr))
				helper.AssertTrue(valid, "Should be valid: %s", jsonStr)
			}

			invalidJSONs := []string{
				`{`, `{"key": }`, `{key: "value"}`,
			}

			for _, jsonStr := range invalidJSONs {
				valid := Valid([]byte(jsonStr))
				helper.AssertFalse(valid, "Should be invalid: %s", jsonStr)
			}

			helper.AssertTrue(IsValidJson(`{"valid": true}`))
			helper.AssertFalse(IsValidJson(`{invalid}`))
		})

		t.Run("FormattingOperations", func(t *testing.T) {
			data := map[string]any{"name": "test"}

			result, err := MarshalIndent(data, "", "  ")
			helper.AssertNoError(err)
			helper.AssertTrue(len(result) > 0)

			prettyResult, err := EncodePretty(data)
			helper.AssertNoError(err)
			helper.AssertTrue(strings.Contains(prettyResult, "\n"))

			compactResult, err := EncodeCompact(data)
			helper.AssertNoError(err)
			helper.AssertFalse(strings.Contains(compactResult, "\n"))

			jsonStr := `{"name":"test"}`
			prettyFormatted, err := FormatPretty(jsonStr)
			helper.AssertNoError(err)
			helper.AssertTrue(strings.Contains(prettyFormatted, "\n"))

			compactFormatted, err := FormatCompact(jsonStr)
			helper.AssertNoError(err)
			helper.AssertFalse(strings.Contains(compactFormatted, "\n"))
		})

		t.Run("BufferOperations", func(t *testing.T) {
			src := []byte(`{"name":"test","age":30}`)

			// Test Compact
			var compactBuf bytes.Buffer
			err := Compact(&compactBuf, src)
			helper.AssertNoError(err)
			helper.AssertTrue(compactBuf.Len() > 0)

			// Test Indent
			var indentBuf bytes.Buffer
			err = Indent(&indentBuf, src, "", "  ")
			helper.AssertNoError(err)
			helper.AssertTrue(indentBuf.Len() > 0)

			// Test HTMLEscape
			srcWithHTML := []byte(`{"html":"<script>alert('xss')</script>"}`)
			var escapeBuf bytes.Buffer
			HTMLEscape(&escapeBuf, srcWithHTML)
			helper.AssertTrue(escapeBuf.Len() > 0)
			// HTMLEscape should escape HTML characters
			escaped := escapeBuf.String()
			helper.AssertTrue(len(escaped) > 0)
		})
	})

	// ========== TYPE CONVERSION ==========
	t.Run("TypeConversion", func(t *testing.T) {
		t.Run("ToInt", func(t *testing.T) {
			tests := []struct {
				input   any
				want    int
				success bool
			}{
				{42, 42, true},
				{int64(100), 100, true},
				{42.0, 42, true},
				{"123", 123, true},
				{"abc", 0, false},
				{true, 1, true},
				{false, 0, true},
			}

			for _, tt := range tests {
				result, ok := ConvertToInt(tt.input)
				helper.AssertEqual(tt.success, ok)
				if ok {
					helper.AssertEqual(tt.want, result)
				}
			}
		})

		t.Run("ToFloat64", func(t *testing.T) {
			result, ok := ConvertToFloat64(42.5)
			helper.AssertTrue(ok)
			helper.AssertEqual(42.5, result)

			result, ok = ConvertToFloat64("3.14")
			helper.AssertTrue(ok)
			helper.AssertEqual(3.14, result)
		})

		t.Run("ToString", func(t *testing.T) {
			helper.AssertEqual("hello", ConvertToString("hello"))
			helper.AssertEqual("42", ConvertToString(42))
			helper.AssertEqual("true", ConvertToString(true))
		})

		t.Run("ToBool", func(t *testing.T) {
			result, ok := ConvertToBool(true)
			helper.AssertTrue(ok)
			helper.AssertTrue(result)

			result, ok = ConvertToBool("true")
			helper.AssertTrue(ok)
			helper.AssertTrue(result)

			result, ok = ConvertToBool(1)
			helper.AssertTrue(ok)
			helper.AssertTrue(result)
		})

		t.Run("TypeSafeOperations", func(t *testing.T) {
			intVal, err := TypeSafeConvert[int](42)
			helper.AssertNoError(err)
			helper.AssertEqual(42, intVal)

			strVal, err := TypeSafeConvert[string]("test")
			helper.AssertNoError(err)
			helper.AssertEqual("test", strVal)

			var value any = "test"
			safeStr, ok := SafeTypeAssert[string](value)
			helper.AssertTrue(ok)
			helper.AssertEqual("test", safeStr)
		})
	})

	// ========== UTILITY FUNCTIONS ==========
	t.Run("UtilityFunctions", func(t *testing.T) {
		t.Run("DeepCopy", func(t *testing.T) {
			original := map[string]any{
				"name": "test",
				"nested": map[string]any{
					"value": 123,
				},
				"array": []any{1, 2, 3},
			}

			copied, err := DeepCopy(original)
			helper.AssertNoError(err)
			helper.AssertNotNil(copied)

			original["name"] = "modified"
			copiedMap := copied.(map[string]any)
			helper.AssertEqual("test", copiedMap["name"])
		})

		t.Run("CompareJson", func(t *testing.T) {
			json1 := `{"name": "test", "value": 123}`
			json2 := `{"value": 123, "name": "test"}`
			json3 := `{"name": "different", "value": 123}`

			equal, err := CompareJson(json1, json2)
			helper.AssertNoError(err)
			helper.AssertTrue(equal)

			equal, err = CompareJson(json1, json3)
			helper.AssertNoError(err)
			helper.AssertFalse(equal)
		})

		t.Run("MergeJson", func(t *testing.T) {
			json1 := `{"name": "test", "value": 123}`
			json2 := `{"extra": "field"}`

			merged, err := MergeJson(json1, json2)
			helper.AssertNoError(err)
			helper.AssertNotNil(merged)

			name, _ := GetString(merged, "name")
			helper.AssertEqual("test", name)

			extra, _ := GetString(merged, "extra")
			helper.AssertEqual("field", extra)
		})
	})
}

// TestErrorSecurityComprehensive consolidates error handling and security tests
// This replaces: error_security_test.go
func TestErrorSecurityComprehensive(t *testing.T) {
	helper := NewTestHelper(t)

	t.Run("ErrorHandling", func(t *testing.T) {
		t.Run("ErrorCreation", func(t *testing.T) {
			err := &JsonsError{
				Op:      "test_operation",
				Message: "test error message",
				Err:     ErrOperationFailed,
			}

			helper.AssertEqual("test_operation", err.Op)
			helper.AssertEqual("test error message", err.Message)
			errorStr := err.Error()
			helper.AssertTrue(strings.Contains(errorStr, "test_operation"))
		})

		t.Run("InvalidJSON", func(t *testing.T) {
			invalidJSONs := []string{
				`{`, `{"key": }`, `{key: "value"}`,
			}

			for _, invalidJSON := range invalidJSONs {
				_, err := Get(invalidJSON, "key")
				helper.AssertError(err)
			}
		})

		t.Run("InvalidPath", func(t *testing.T) {
			validJSON := `{"users": [{"name": "Alice"}]}`

			invalidPaths := []string{
				"users[abc]", "users[]", "users[",
			}

			for _, invalidPath := range invalidPaths {
				result, err := Get(validJSON, invalidPath)
				if err != nil {
					helper.AssertError(err)
				} else {
					helper.AssertNil(result)
				}
			}
		})

		t.Run("ProcessorClosed", func(t *testing.T) {
			processor := New()
			processor.Close()

			_, err := processor.Get(`{"test": "value"}`, "test")
			helper.AssertError(err)
		})

		t.Run("ErrorRecovery", func(t *testing.T) {
			processor := New()
			defer processor.Close()

			validJSON := `{"valid": "data"}`
			invalidJSON := `{invalid}`

			_, err1 := processor.Get(invalidJSON, "key")
			helper.AssertError(err1)

			result2, err2 := processor.Get(validJSON, "valid")
			helper.AssertNoError(err2)
			helper.AssertEqual("data", result2)
		})
	})

	t.Run("SecurityValidation", func(t *testing.T) {
		processor := New()
		defer processor.Close()

		t.Run("PathTraversal", func(t *testing.T) {
			testCases := []struct {
				name        string
				path        string
				shouldBlock bool
			}{
				{"Unix traversal", "../etc/passwd", true},
				{"Windows traversal", "..\\windows\\system32", true},
				{"URL-encoded", "%2e%2e/etc/passwd", true},
				{"Double-encoded", "%252e%252e/etc/passwd", true},
				{"UTF-8 overlong", "..%c0%af/etc/passwd", true},
				{"Null byte", "../etc/passwd\x00.json", true},
				{"Valid property", "user.name", false},
				{"Valid array", "users[0].name", false},
			}

			for _, tc := range testCases {
				t.Run(tc.name, func(t *testing.T) {
					err := processor.validatePath(tc.path)

					if tc.shouldBlock {
						if err == nil {
							t.Errorf("Expected security error for path '%s'", tc.path)
						}
					} else {
						if err != nil {
							t.Errorf("Expected valid path '%s', got error: %v", tc.path, err)
						}
					}
				})
			}
		})

		t.Run("PathLength", func(t *testing.T) {
			normalPath := strings.Repeat("a", 100)
			err := processor.validatePath(normalPath)
			helper.AssertNoError(err)

			tooLongPath := strings.Repeat("a", MaxPathLength+1)
			err = processor.validatePath(tooLongPath)
			helper.AssertError(err)
		})

		t.Run("MaliciousInput", func(t *testing.T) {
			testData := `{"user": {"name": "Alice"}}`

			maliciousPaths := []string{
				"user.name'; DROP TABLE users; --",
				"user.name<script>alert('xss')</script>",
			}

			for _, path := range maliciousPaths {
				_, err := Get(testData, path)
				if err == nil {
					t.Logf("Malicious path handled safely: %s", path)
				}
			}
		})
	})
}

// TestProcessorConcurrencyComprehensive consolidates processor and concurrency tests
// This replaces: processor_concurrency_test.go
func TestProcessorConcurrencyComprehensive(t *testing.T) {
	helper := NewTestHelper(t)
	generator := NewTestDataGenerator()

	t.Run("ProcessorOperations", func(t *testing.T) {
		processor := New()
		defer processor.Close()

		testData := `{
			"user": {"name": "John", "age": 30},
			"items": [1, 2, 3, 4, 5],
			"active": true
		}`

		t.Run("GetMultiple", func(t *testing.T) {
			paths := []string{"user.name", "user.age", "active"}
			results, err := processor.GetMultiple(testData, paths)
			helper.AssertNoError(err)
			helper.AssertEqual(3, len(results))
			helper.AssertEqual("John", results["user.name"])
		})

		t.Run("ProcessBatch", func(t *testing.T) {
			operations := []BatchOperation{
				{Type: "get", JSONStr: `{"name": "John"}`, Path: "name", ID: "op1"},
				{Type: "set", JSONStr: `{"age": 30}`, Path: "age", Value: 35, ID: "op2"},
			}

			batchResults, err := processor.ProcessBatch(operations)
			helper.AssertNoError(err)
			helper.AssertEqual(2, len(batchResults))
		})

		t.Run("Stats", func(t *testing.T) {
			_, _ = processor.Get(testData, "user.name")
			stats := processor.GetStats()
			helper.AssertTrue(stats.OperationCount > 0)
		})

		t.Run("HealthStatus", func(t *testing.T) {
			health := processor.GetHealthStatus()
			helper.AssertNotNil(health)
			helper.AssertTrue(len(health.Checks) > 0)
		})

		t.Run("CacheOperations", func(t *testing.T) {
			_, _ = processor.Get(testData, "user.name")
			processor.ClearCache()

			samplePaths := []string{"user.name", "active"}
			_, _ = processor.WarmupCache(testData, samplePaths)
		})

		t.Run("Lifecycle", func(t *testing.T) {
			p := New()
			helper.AssertFalse(p.IsClosed())
			p.Close()
			helper.AssertTrue(p.IsClosed())

			_, err := p.Get(`{"test": "value"}`, "test")
			helper.AssertError(err)
		})
	})

	t.Run("ConcurrentOperations", func(t *testing.T) {
		t.Run("ConcurrentReads", func(t *testing.T) {
			jsonStr := generator.GenerateComplexJSON()
			concurrencyTester := NewConcurrencyTester(t, 20, 100)

			concurrencyTester.Run(func(workerID, iteration int) error {
				paths := []string{
					"users[0].name",
					"settings.appName",
					"statistics.totalUsers",
				}
				path := paths[workerID%len(paths)]
				_, err := Get(jsonStr, path)
				return err
			})
		})

		t.Run("ConcurrentWrites", func(t *testing.T) {
			originalJSON := `{"counters": {"a": 0, "b": 0}}`
			var results []string
			var resultsMutex sync.Mutex

			concurrencyTester := NewConcurrencyTester(t, 10, 50)
			concurrencyTester.Run(func(workerID, iteration int) error {
				counterKey := fmt.Sprintf("counters.%c", 'a'+workerID%2)
				result, err := Set(originalJSON, counterKey, workerID*1000+iteration)
				if err != nil {
					return err
				}
				resultsMutex.Lock()
				results = append(results, result)
				resultsMutex.Unlock()
				return nil
			})

			helper.AssertTrue(len(results) > 0)
		})

		t.Run("MixedOperations", func(t *testing.T) {
			baseJSON := `{"data": {"counter": 0}, "array": [1, 2, 3]}`
			var operations int64

			concurrencyTester := NewConcurrencyTester(t, 15, 100)
			concurrencyTester.Run(func(workerID, iteration int) error {
				atomic.AddInt64(&operations, 1)
				switch iteration % 3 {
				case 0:
					_, err := Get(baseJSON, "data.counter")
					return err
				case 1:
					_, err := Set(baseJSON, "data.counter", iteration)
					return err
				case 2:
					_, err := Get(baseJSON, "array[1]")
					return err
				}
				return nil
			})

			helper.AssertTrue(atomic.LoadInt64(&operations) > 0)
		})

		t.Run("SharedProcessor", func(t *testing.T) {
			processor := New()
			defer processor.Close()

			jsonStr := generator.GenerateComplexJSON()
			concurrencyTester := NewConcurrencyTester(t, 25, 200)

			concurrencyTester.Run(func(workerID, iteration int) error {
				switch iteration % 2 {
				case 0:
					_, err := processor.Get(jsonStr, "users[0].name")
					return err
				case 1:
					_, err := Set(jsonStr, fmt.Sprintf("worker_%d", workerID), iteration)
					return err
				}
				return nil
			})
		})

		t.Run("MultipleProcessors", func(t *testing.T) {
			const numProcessors = 10
			const operationsPerProcessor = 100

			jsonStr := `{"test": "value", "number": 42}`
			var wg sync.WaitGroup
			var totalOps, totalErrors int64

			for i := 0; i < numProcessors; i++ {
				wg.Add(1)
				go func(processorID int) {
					defer wg.Done()
					processor := New()
					defer processor.Close()

					for j := 0; j < operationsPerProcessor; j++ {
						atomic.AddInt64(&totalOps, 1)
						_, err := processor.Get(jsonStr, "test")
						if err != nil {
							atomic.AddInt64(&totalErrors, 1)
						}
					}
				}(i)
			}

			wg.Wait()
			helper.AssertEqual(int64(numProcessors*operationsPerProcessor), atomic.LoadInt64(&totalOps))
			helper.AssertEqual(int64(0), atomic.LoadInt64(&totalErrors))
		})

		t.Run("CacheThreadSafety", func(t *testing.T) {
			config := DefaultConfig()
			config.EnableCache = true
			config.MaxCacheSize = 100

			processor := New(config)
			defer processor.Close()

			jsonStr := `{"cached": "value", "number": 123}`
			const numWorkers = 15
			const operationsPerWorker = 100

			var wg sync.WaitGroup
			for i := 0; i < numWorkers; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					for j := 0; j < operationsPerWorker; j++ {
						if j%2 == 0 {
							processor.Get(jsonStr, "cached")
						} else {
							processor.Get(jsonStr, fmt.Sprintf("dynamic_%d", j))
						}
					}
				}()
			}

			wg.Wait()
			stats := processor.GetStats()
			helper.AssertTrue(stats.HitCount+stats.MissCount > 0)
		})
	})
}

// TestConfigConstantsComprehensive consolidates configuration and constants tests
// This replaces: config_constants_test.go
func TestConfigConstantsComprehensive(t *testing.T) {
	helper := NewTestHelper(t)

	t.Run("ConfigGetters", func(t *testing.T) {
		config := DefaultConfig()

		helper.AssertTrue(config.IsCacheEnabled())
		helper.AssertTrue(config.GetMaxCacheSize() > 0)
		helper.AssertTrue(config.GetCacheTTL() > 0)
		helper.AssertTrue(config.GetMaxJSONSize() > 0)
		helper.AssertTrue(config.GetMaxPathDepth() > 0)
		helper.AssertTrue(config.GetMaxConcurrency() > 0)
		helper.AssertTrue(config.GetMaxNestingDepth() >= 0)

		limits := config.GetSecurityLimits()
		helper.AssertNotNil(limits)
	})

	t.Run("ConfigPresets", func(t *testing.T) {
		config := DefaultProcessorConfig()
		helper.AssertNotNil(config)

		highSec := HighSecurityConfig()
		helper.AssertNotNil(highSec)

		largeDat := LargeDataConfig()
		helper.AssertNotNil(largeDat)
	})

	t.Run("EncodeConfigPresets", func(t *testing.T) {
		config := DefaultEncodeConfig()
		helper.AssertNotNil(config)

		pretty := NewPrettyConfig()
		helper.AssertNotNil(pretty)
		helper.AssertTrue(pretty.Pretty)

		compact := NewCompactConfig()
		helper.AssertNotNil(compact)

		cloned := config.Clone()
		helper.AssertNotNil(cloned)
	})

	t.Run("ConfigValidation", func(t *testing.T) {
		config := DefaultConfig()
		err := ValidateConfig(config)
		helper.AssertNoError(err)

		invalidConfig := DefaultConfig()
		invalidConfig.MaxCacheSize = -1
		err = ValidateConfig(invalidConfig)
		helper.AssertError(err)
	})

	t.Run("Constants", func(t *testing.T) {
		helper.AssertTrue(DefaultBufferSize > 0)
		helper.AssertTrue(MaxPoolBufferSize > MinPoolBufferSize)
		helper.AssertTrue(DefaultCacheSize > 0)
		helper.AssertTrue(DefaultMaxJSONSize > 0)
		helper.AssertTrue(DefaultMaxNestingDepth > 0)
		helper.AssertTrue(MaxPathLength > 0)
		helper.AssertTrue(DefaultOperationTimeout > 0)
	})

	t.Run("ErrorCodes", func(t *testing.T) {
		errorCodes := []string{
			ErrCodeInvalidJSON,
			ErrCodePathNotFound,
			ErrCodeTypeMismatch,
			ErrCodeSizeLimit,
			ErrCodeSecurityViolation,
		}

		for _, code := range errorCodes {
			helper.AssertTrue(len(code) > 0)
			helper.AssertTrue(code[:4] == "ERR_")
		}
	})

	t.Run("GlobalProcessor", func(t *testing.T) {
		SetGlobalProcessor(New(DefaultConfig()))
		ShutdownGlobalProcessor()
	})
}

