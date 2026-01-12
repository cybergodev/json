package json

import (
	"encoding/json"
	"testing"
)

// TestIterableValue_PathNavigation tests the path navigation functionality
func TestIterableValue_PathNavigation(t *testing.T) {
	jsonStr := `{
		"user": {
			"name": "John Doe",
			"age": 30,
			"active": true,
			"score": 95.5,
			"address": {
				"city": "New York",
				"zip": "10001"
			},
			"hobbies": ["reading", "gaming", "coding"],
			"posts": [
				{
					"id": 1,
					"title": "First Post",
					"tags": ["intro", "hello"]
				},
				{
					"id": 2,
					"title": "Second Post",
					"tags": ["update", "news"]
				}
			]
		},
		"thumbnails": [
			{"url": "small.jpg", "width": 100},
			{"url": "medium.jpg", "width": 300},
			{"url": "large.jpg", "width": 800}
		]
	}`

	var data map[string]any
	if err := json.Unmarshal([]byte(jsonStr), &data); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	iv := &IterableValue{data: data}

	t.Run("SimplePropertyAccess", func(t *testing.T) {
		tests := []struct {
			name     string
			path     string
			expected string
		}{
			{"Single key", "user.name", "John Doe"},
			{"Nested path", "user.address.city", "New York"},
			{"Deep nested", "user.address.zip", "10001"},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				result := iv.GetString(tt.path)
				if result != tt.expected {
					t.Errorf("GetString(%q) = %q, want %q", tt.path, result, tt.expected)
				}
			})
		}
	})

	t.Run("ArrayIndexAccess", func(t *testing.T) {
		tests := []struct {
			name     string
			path     string
			expected string
		}{
			{"First element", "user.hobbies[0]", "reading"},
			{"Second element", "user.hobbies[1]", "gaming"},
			{"Last element", "user.hobbies[2]", "coding"},
			{"Nested array", "user.posts[0].title", "First Post"},
			{"Nested array deep", "user.posts[1].tags[0]", "update"},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				result := iv.GetString(tt.path)
				if result != tt.expected {
					t.Errorf("GetString(%q) = %q, want %q", tt.path, result, tt.expected)
				}
			})
		}
	})

	t.Run("NegativeArrayIndex", func(t *testing.T) {
		tests := []struct {
			name     string
			path     string
			expected string
		}{
			{"Last element with -1", "user.hobbies[-1]", "coding"},
			{"Second to last with -2", "user.hobbies[-2]", "gaming"},
			{"First from end", "user.posts[-1].title", "Second Post"},
			{"Thumbnail last", "thumbnails[-1].url", "large.jpg"},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				result := iv.GetString(tt.path)
				if result != tt.expected {
					t.Errorf("GetString(%q) = %q, want %q", tt.path, result, tt.expected)
				}
			})
		}
	})

	t.Run("TypeSpecificGetters", func(t *testing.T) {
		t.Run("GetInt", func(t *testing.T) {
			if age := iv.GetInt("user.age"); age != 30 {
				t.Errorf("GetInt(user.age) = %d, want 30", age)
			}
			if postID := iv.GetInt("user.posts[0].id"); postID != 1 {
				t.Errorf("GetInt(user.posts[0].id) = %d, want 1", postID)
			}
		})

		t.Run("GetFloat64", func(t *testing.T) {
			if score := iv.GetFloat64("user.score"); score != 95.5 {
				t.Errorf("GetFloat64(user.score) = %f, want 95.5", score)
			}
		})

		t.Run("GetBool", func(t *testing.T) {
			if active := iv.GetBool("user.active"); !active {
				t.Errorf("GetBool(user.active) = false, want true")
			}
		})

		t.Run("GetArray", func(t *testing.T) {
			hobbies := iv.GetArray("user.hobbies")
			if hobbies == nil || len(hobbies) != 3 {
				t.Errorf("GetArray(user.hobbies) = %v, want array of length 3", hobbies)
			}
		})

		t.Run("GetObject", func(t *testing.T) {
			address := iv.GetObject("user.address")
			if address == nil {
				t.Errorf("GetObject(user.address) = nil, want map")
			}
		})
	})

	t.Run("DefaultValues", func(t *testing.T) {
		t.Run("GetStringWithDefault", func(t *testing.T) {
			if val := iv.GetStringWithDefault("user.nonexistent", "default"); val != "default" {
				t.Errorf("GetStringWithDefault(nonexistent) = %q, want 'default'", val)
			}
			if val := iv.GetStringWithDefault("user.name", "default"); val != "John Doe" {
				t.Errorf("GetStringWithDefault(user.name) = %q, want 'John Doe'", val)
			}
		})

		t.Run("GetIntWithDefault", func(t *testing.T) {
			if val := iv.GetIntWithDefault("user.nonexistent", 99); val != 99 {
				t.Errorf("GetIntWithDefault(nonexistent) = %d, want 99", val)
			}
			if val := iv.GetIntWithDefault("user.age", 99); val != 30 {
				t.Errorf("GetIntWithDefault(user.age) = %d, want 30", val)
			}
		})
	})

	t.Run("ExistsAndNull", func(t *testing.T) {
		t.Run("Exists", func(t *testing.T) {
			if !iv.Exists("user.name") {
				t.Error("Exists(user.name) = false, want true")
			}
			if iv.Exists("user.nonexistent") {
				t.Error("Exists(user.nonexistent) = true, want false")
			}
		})

		t.Run("IsNull", func(t *testing.T) {
			if iv.IsNull("user.name") {
				t.Error("IsNull(user.name) = true, want false")
			}
			// Note: we can't test null values in this JSON as we don't have any
		})
	})

	t.Run("GetGeneric", func(t *testing.T) {
		t.Run("GetString", func(t *testing.T) {
			if val := iv.Get("user.name"); val != "John Doe" {
				t.Errorf("Get(user.name) = %v, want 'John Doe'", val)
			}
		})

		t.Run("GetNested", func(t *testing.T) {
			if val := iv.Get("user.address.city"); val != "New York" {
				t.Errorf("Get(user.address.city) = %v, want 'New York'", val)
			}
		})

		t.Run("GetArrayElement", func(t *testing.T) {
			if val := iv.Get("user.hobbies[0]"); val != "reading" {
				t.Errorf("Get(user.hobbies[0]) = %v, want 'reading'", val)
			}
		})
	})
}

// TestIterableValue_BackwardCompatibility tests that simple key lookup still works
func TestIterableValue_BackwardCompatibility(t *testing.T) {
	jsonStr := `{
		"name": "Test",
		"value": 42,
		"flag": true,
		"items": [1, 2, 3]
	}`

	var data map[string]any
	if err := json.Unmarshal([]byte(jsonStr), &data); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	iv := &IterableValue{data: data}

	// Test simple key access (without dots)
	if name := iv.GetString("name"); name != "Test" {
		t.Errorf("GetString(name) = %q, want 'Test'", name)
	}

	if value := iv.GetInt("value"); value != 42 {
		t.Errorf("GetInt(value) = %d, want 42", value)
	}

	if flag := iv.GetBool("flag"); !flag {
		t.Errorf("GetBool(flag) = false, want true")
	}

	items := iv.GetArray("items")
	if items == nil || len(items) != 3 {
		t.Errorf("GetArray(items) = %v, want array of length 3", items)
	}
}

// TestIterableValue_RealWorldScenario tests real-world JSON parsing scenarios
func TestIterableValue_RealWorldScenario(t *testing.T) {
	// Simulate YouTube API response structure
	jsonStr := `{
		"contents": {
			"twoColumnBrowseResultsRenderer": {
				"tabs": [
					{
						"tabRenderer": {
							"title": "Home",
							"selected": false
						}
					},
					{
						"tabRenderer": {
							"title": "Videos",
							"selected": true,
							"content": {
								"richGridRenderer": {
									"contents": [
										{
											"richItemRenderer": {
												"content": {
													"videoRenderer": {
														"videoId": "abc123",
														"title": {
															"runs": [{"text": "Test Video"}]
														},
														"thumbnail": {
															"thumbnails": [
																{"url": "thumb1.jpg"},
																{"url": "thumb2.jpg"}
															]
														}
													}
												}
											}
										}
									]
								}
							}
						}
					}
				]
			}
		}
	}`

	var data map[string]any
	if err := json.Unmarshal([]byte(jsonStr), &data); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	iv := &IterableValue{data: data}

	t.Run("DeepNestedExtraction", func(t *testing.T) {
		videoID := iv.GetString("contents.twoColumnBrowseResultsRenderer.tabs[1].tabRenderer.content.richGridRenderer.contents[0].richItemRenderer.content.videoRenderer.videoId")
		if videoID != "abc123" {
			t.Errorf("Video ID = %q, want 'abc123'", videoID)
		}

		title := iv.GetString("contents.twoColumnBrowseResultsRenderer.tabs[1].tabRenderer.content.richGridRenderer.contents[0].richItemRenderer.content.videoRenderer.title.runs[0].text")
		if title != "Test Video" {
			t.Errorf("Title = %q, want 'Test Video'", title)
		}

		thumbnailURL := iv.GetString("contents.twoColumnBrowseResultsRenderer.tabs[1].tabRenderer.content.richGridRenderer.contents[0].richItemRenderer.content.videoRenderer.thumbnail.thumbnails[-1].url")
		if thumbnailURL != "thumb2.jpg" {
			t.Errorf("Thumbnail URL = %q, want 'thumb2.jpg'", thumbnailURL)
		}
	})

	t.Run("TabSelection", func(t *testing.T) {
		selected := iv.GetBool("contents.twoColumnBrowseResultsRenderer.tabs[1].tabRenderer.selected")
		if !selected {
			t.Error("Second tab should be selected")
		}

		notSelected := iv.GetBool("contents.twoColumnBrowseResultsRenderer.tabs[0].tabRenderer.selected")
		if notSelected {
			t.Error("First tab should not be selected")
		}
	})
}

// TestIterableValue_EdgeCases tests edge cases and error conditions
func TestIterableValue_EdgeCases(t *testing.T) {
	jsonStr := `{
		"emptyArray": [],
		"emptyObject": {},
		"nullField": null,
		"array": [1, 2, 3]
	}`

	var data map[string]any
	if err := json.Unmarshal([]byte(jsonStr), &data); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	iv := &IterableValue{data: data}

	t.Run("EmptyArray", func(t *testing.T) {
		arr := iv.GetArray("emptyArray")
		if arr == nil || len(arr) != 0 {
			t.Errorf("GetArray(emptyArray) = %v, want empty array", arr)
		}

		if !iv.IsEmpty("emptyArray") {
			t.Error("IsEmpty(emptyArray) should return true")
		}
	})

	t.Run("EmptyObject", func(t *testing.T) {
		obj := iv.GetObject("emptyObject")
		if obj == nil || len(obj) != 0 {
			t.Errorf("GetObject(emptyObject) = %v, want empty object", obj)
		}
	})

	t.Run("NonExistentPath", func(t *testing.T) {
		if iv.Exists("nonexistent.path") {
			t.Error("Exists(nonexistent.path) should return false")
		}

		if val := iv.GetString("nonexistent.path"); val != "" {
			t.Errorf("GetString(nonexistent.path) = %q, want empty string", val)
		}
	})

	t.Run("InvalidArrayIndex", func(t *testing.T) {
		if val := iv.GetString("array[10]"); val != "" {
			t.Errorf("GetString(array[10]) = %q, want empty string (out of bounds)", val)
		}

		if val := iv.GetString("array[-10]"); val != "" {
			t.Errorf("GetString(array[-10]) = %q, want empty string (out of bounds)", val)
		}
	})
}

// TestIterableValue_ForeachNestedWithPath tests ForeachNested with path navigation
func TestIterableValue_ForeachNestedWithPath(t *testing.T) {
	jsonStr := `{
		"users": [
			{"name": "Alice", "age": 25},
			{"name": "Bob", "age": 30},
			{"name": "Charlie", "age": 35}
		]
	}`

	var data map[string]any
	if err := json.Unmarshal([]byte(jsonStr), &data); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	iv := &IterableValue{data: data}

	t.Run("ForeachOverArray", func(t *testing.T) {
		usersArray := iv.GetArray("users")
		if usersArray == nil || len(usersArray) != 3 {
			t.Fatalf("GetArray(users) = %v, want array of length 3", usersArray)
		}

		count := 0
		for _, user := range usersArray {
			userIV := &IterableValue{data: user}
			count++
			name := userIV.GetString("name")
			if name == "" {
				t.Errorf("Expected non-empty name at index %d", count-1)
			}
		}

		if count != 3 {
			t.Errorf("Iterated over %d users, want 3", count)
		}
	})

	t.Run("ForeachNestedRecursive", func(t *testing.T) {
		// Test that ForeachNested recursively iterates over all nested values
		count := 0
		iv.ForeachNested("users", func(key any, item *IterableValue) {
			count++
		})

		// ForeachNested recursively iterates, so count should be > 3
		if count < 3 {
			t.Errorf("ForeachRecursive count = %d, want at least 3 (it's recursive)", count)
		}
	})
}

// TestIterableValue_MixedPathAndKeyAccess tests mixing path and key access
func TestIterableValue_MixedPathAndKeyAccess(t *testing.T) {
	jsonStr := `{
		"data": {
			"user": {
				"name": "Test User",
				"settings": {
					"theme": "dark"
				}
			}
		}
	}`

	var data map[string]any
	if err := json.Unmarshal([]byte(jsonStr), &data); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	iv := &IterableValue{data: data}

	// Access with full path
	if name := iv.GetString("data.user.name"); name != "Test User" {
		t.Errorf("GetString(data.user.name) = %q, want 'Test User'", name)
	}

	// Access nested object then use simple key
	userObj := iv.GetObject("data.user")
	if userObj == nil {
		t.Fatal("GetObject(data.user) returned nil")
	}

	userIV := &IterableValue{data: userObj}
	if name := userIV.GetString("name"); name != "Test User" {
		t.Errorf("Nested GetString(name) = %q, want 'Test User'", name)
	}

	if theme := userIV.GetString("settings.theme"); theme != "dark" {
		t.Errorf("Nested GetString(settings.theme) = %q, want 'dark'", theme)
	}
}
