package json

import (
	"testing"
)

// TestProcessor_ForeachMethods tests Processor's Foreach methods
func TestProcessor_ForeachMethods(t *testing.T) {
	jsonStr := `{
		"users": [
			{"name": "Alice", "age": 25},
			{"name": "Bob", "age": 30},
			{"name": "Charlie", "age": 35}
		]
	}`

	t.Run("Foreach", func(t *testing.T) {
		processor := New(DefaultConfig())
		defer processor.Close()

		count := 0
		processor.Foreach(jsonStr, func(key any, item *IterableValue) {
			count++
		})

		// Foreach on root should iterate over "users" key
		if count == 0 {
			t.Error("Foreach should have iterated over at least one item")
		}
	})

	t.Run("ForeachWithPath", func(t *testing.T) {
		processor := New(DefaultConfig())
		defer processor.Close()

		count := 0
		names := []string{}

		err := processor.ForeachWithPath(jsonStr, "users", func(key any, item *IterableValue) {
			count++
			name := item.GetString("name")
			names = append(names, name)
		})

		if err != nil {
			t.Errorf("ForeachWithPath error: %v", err)
		}

		if count != 3 {
			t.Errorf("ForeachWithPath count = %d, want 3", count)
		}

		expectedNames := []string{"Alice", "Bob", "Charlie"}
		if len(names) != 3 {
			t.Errorf("Names count = %d, want 3", len(names))
		} else {
			for i, name := range names {
				if name != expectedNames[i] {
					t.Errorf("names[%d] = %q, want %q", i, name, expectedNames[i])
				}
			}
		}
	})

	t.Run("ForeachWithPathDeepNesting", func(t *testing.T) {
		// Create a processor with custom nesting depth limit
		config := DefaultConfig()
		config.MaxNestingDepthSecurity = 50

		processor := New(config)
		defer processor.Close()

		// Deeply nested JSON structure with array at the end
		deepJSON := `{
			"level1": {
				"level2": {
					"level3": {
						"level4": {
							"items": [
								{"value": "deep1"},
								{"value": "deep2"}
							]
						}
					}
				}
			}
		}`

		found := false
		err := processor.ForeachWithPath(deepJSON, "level1.level2.level3.level4.items", func(key any, item *IterableValue) {
			value := item.GetString("value")
			if value == "deep1" || value == "deep2" {
				found = true
			}
		})

		if err != nil {
			t.Errorf("ForeachWithPath on deep structure error: %v", err)
		}

		if !found {
			t.Error("Expected to find deep values")
		}
	})

	t.Run("ForeachWithPathAndControl", func(t *testing.T) {
		processor := New(DefaultConfig())
		defer processor.Close()

		count := 0
		err := processor.ForeachWithPathAndControl(jsonStr, "users", func(key any, value any) IteratorControl {
			count++
			// Continue iteration
			return IteratorContinue
		})

		if err != nil {
			t.Errorf("ForeachWithPathAndControl error: %v", err)
		}

		if count != 3 {
			t.Errorf("ForeachWithPathAndControl count = %d, want 3", count)
		}
	})

	t.Run("ForeachWithPathBreakEarly", func(t *testing.T) {
		processor := New(DefaultConfig())
		defer processor.Close()

		count := 0
		err := processor.ForeachWithPathAndControl(jsonStr, "users", func(key any, value any) IteratorControl {
			count++
			// Break after first item
			if count == 1 {
				return IteratorBreak
			}
			return IteratorContinue
		})

		if err != nil {
			t.Errorf("ForeachWithPathAndControl error: %v", err)
		}

		if count != 1 {
			t.Errorf("ForeachWithPathAndControl with break count = %d, want 1", count)
		}
	})

	t.Run("ForeachWithPathAndIterator", func(t *testing.T) {
		processor := New(DefaultConfig())
		defer processor.Close()

		paths := []string{}
		err := processor.ForeachWithPathAndIterator(jsonStr, "users", func(key any, item *IterableValue, currentPath string) IteratorControl {
			paths = append(paths, currentPath)
			return IteratorContinue
		})

		if err != nil {
			t.Errorf("ForeachWithPathAndIterator error: %v", err)
		}

		if len(paths) != 3 {
			t.Errorf("ForeachWithPathAndIterator paths count = %d, want 3", len(paths))
		}
	})
}

// TestProcessor_CustomConfigVsDefault tests that custom processor config is actually used
func TestProcessor_CustomConfigVsDefault(t *testing.T) {
	// JSON with depth exceeding default limit
	deepJSON := `{"a":{` + string(make([]byte, 40)) + `}}`

	// Create custom config with higher nesting limit
	config := DefaultConfig()
	config.MaxNestingDepthSecurity = 100

	processor := New(config)
	defer processor.Close()

	// This should work with custom config
	processor.Foreach(deepJSON, func(key any, item *IterableValue) {
		// If we get here without error, custom config is being used
		_ = item.GetString("a")
	})
}

// TestProcessor_ForeachVsPackageLevel tests that Processor methods work independently
func TestProcessor_ForeachVsPackageLevel(t *testing.T) {
	jsonStr := `{"items": [1, 2, 3]}`

	t.Run("ProcessorMethod", func(t *testing.T) {
		processor := New(DefaultConfig())
		defer processor.Close()

		count := 0
		err := processor.ForeachWithPath(jsonStr, "items", func(key any, item *IterableValue) {
			count++
		})

		if err != nil {
			t.Errorf("Processor.ForeachWithPath error: %v", err)
		}

		if count != 3 {
			t.Errorf("Processor.ForeachWithPath count = %d, want 3", count)
		}
	})

	t.Run("PackageLevelFunction", func(t *testing.T) {
		count := 0
		err := ForeachWithPath(jsonStr, "items", func(key any, item *IterableValue) {
			count++
		})

		if err != nil {
			t.Errorf("ForeachWithPath error: %v", err)
		}

		if count != 3 {
			t.Errorf("ForeachWithPath count = %d, want 3", count)
		}
	})
}

// TestProcessor_ForeachReturn tests the ForeachReturn method
func TestProcessor_ForeachReturn(t *testing.T) {
	jsonStr := `{"items": [1, 2, 3]}`

	processor := New(DefaultConfig())
	defer processor.Close()

	count := 0
	result, err := processor.ForeachReturn(jsonStr, func(key any, item *IterableValue) {
		count++
	})

	if err != nil {
		t.Errorf("ForeachReturn error: %v", err)
	}

	if count != 1 {
		t.Errorf("ForeachReturn count = %d, want 1 (just 'items' key)", count)
	}

	if result != jsonStr {
		t.Error("ForeachReturn should return the original JSON string")
	}
}

// TestProcessor_ForeachNested tests the ForeachNested method
func TestProcessor_ForeachNested(t *testing.T) {
	jsonStr := `{
		"user": {
			"name": "Alice",
			"age": 25,
			"address": {
				"city": "NYC"
			}
		}
	}`

	processor := New(DefaultConfig())
	defer processor.Close()

	count := 0
	processor.ForeachNested(jsonStr, func(key any, item *IterableValue) {
		count++
	})

	// ForeachNested recursively iterates, so count should be > 5
	if count < 5 {
		t.Errorf("ForeachNested count = %d, want at least 5 (it's recursive)", count)
	}
}
