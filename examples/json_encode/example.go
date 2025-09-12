package main

// ==================================================================================
// This is a comprehensive example of JSON encoding functionality
//
// This example demonstrates different approaches to encoding Go values to JSON:
// 1. Basic encoding using json.Encode() - converts any Go value to JSON string
// 2. Pretty printing using json.EncodePretty() - formatted JSON with indentation
// 3. Compact encoding using json.EncodeCompact() - minimal JSON without whitespace
// 4. Custom configuration encoding - fine-grained control over output format
// 5. Predefined configurations - web-safe, readable, and other preset formats
// 6. Advanced encoding options - custom escaping, number formatting, key sorting
// 7. Struct encoding with JSON tags - controlling field names and omission
// 8. Complex data structure encoding - nested objects, arrays, maps
// 9. Custom type encoding - implementing custom JSON marshaling
// 10. Performance optimization - processor reuse and batch operations
//
// The example covers various Go data types including:
// - Basic types (string, int, float, bool)
// - Complex types (struct, map, slice, array)
// - Nested structures and arrays
// - Custom types with JSON marshaling
// - Pointers and nil values
//
// Key features demonstrated:
// - Multiple encoding configurations (pretty, compact, web-safe, readable)
// - Custom escaping and character handling
// - Number format preservation and precision control
// - Key sorting and empty value omission
// - HTML escaping and Unicode handling
// - Performance optimization techniques
// ==================================================================================

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/cybergodev/json"
)

// Sample data structures for encoding examples

type User struct {
	ID       int       `json:"id"`
	Name     string    `json:"name"`
	Email    string    `json:"email"`
	Age      int       `json:"age"`
	Active   bool      `json:"active"`
	Salary   float64   `json:"salary"`
	Tags     []string  `json:"tags,omitempty"`
	Metadata *Metadata `json:"metadata,omitempty"`
	JoinedAt time.Time `json:"joined_at"`
}

type Metadata struct {
	Department string            `json:"department"`
	Level      string            `json:"level"`
	Skills     []string          `json:"skills"`
	Projects   []Project         `json:"projects"`
	Settings   map[string]any    `json:"settings"`
	Attributes map[string]string `json:"attributes,omitempty"`
}

type Project struct {
	Name        string     `json:"name"`
	Status      string     `json:"status"`
	Priority    int        `json:"priority"`
	StartDate   time.Time  `json:"start_date"`
	EndDate     *time.Time `json:"end_date,omitempty"`
	Budget      float64    `json:"budget"`
	TeamMembers []string   `json:"team_members"`
}

type Company struct {
	Name        string       `json:"name"`
	Founded     int          `json:"founded"`
	Employees   []User       `json:"employees"`
	Departments []Department `json:"departments"`
	Config      Config       `json:"config"`
	Revenue     *float64     `json:"revenue,omitempty"`
}

type Department struct {
	Name      string  `json:"name"`
	Budget    float64 `json:"budget"`
	Manager   *User   `json:"manager,omitempty"`
	Employees []User  `json:"employees"`
}

type Config struct {
	Debug       bool              `json:"debug"`
	CacheTTL    int               `json:"cache_ttl"`
	Features    map[string]bool   `json:"features"`
	Limits      map[string]int    `json:"limits"`
	Environment string            `json:"environment"`
	Secrets     map[string]string `json:"secrets,omitempty"`
}

// Custom type with JSON marshaling
type CustomID struct {
	Prefix string
	Number int
}

// MarshalJSON implements custom JSON marshaling for CustomID
func (c CustomID) MarshalJSON() ([]byte, error) {
	return []byte(fmt.Sprintf(`"%s-%04d"`, c.Prefix, c.Number)), nil
}

// UnmarshalJSON implements custom JSON unmarshaling for CustomID
func (c *CustomID) UnmarshalJSON(data []byte) error {
	// This is just for completeness, not used in this example
	return nil
}

type ProductWithCustomID struct {
	ID          CustomID `json:"id"`
	Name        string   `json:"name"`
	Price       float64  `json:"price"`
	InStock     bool     `json:"in_stock"`
	Description *string  `json:"description,omitempty"`
}

func main() {
	// Basic encoding examples
	printLines("=== Basic Encoding Examples ===")
	basicEncoding()

	// Pretty printing examples
	printLines("=== Pretty Printing Examples ===")
	prettyPrinting()

	// Compact encoding examples
	printLines("=== Compact Encoding Examples ===")
	compactEncoding()

	// Custom configuration examples
	printLines("=== Custom Configuration Examples ===")
	customConfiguration()

	// Predefined configuration examples
	printLines("=== Predefined Configuration Examples ===")
	predefinedConfigurations()

	// Advanced encoding options
	printLines("=== Advanced Encoding Options ===")
	advancedEncodingOptions()

	// Struct encoding with JSON tags
	printLines("=== Struct Encoding with JSON Tags ===")
	structEncodingWithTags()

	// Complex data structure encoding
	printLines("=== Complex Data Structure Encoding ===")
	complexDataStructureEncoding()

	// Custom type encoding
	printLines("=== Custom Type Encoding ===")
	customTypeEncoding()

	// Performance optimization
	printLines("=== Performance Optimization ===")
	performanceOptimization()

	// Error handling examples
	printLines("=== Error Handling Examples ===")
	errorHandlingExamples()

	// Null value handling examples
	printLines("=== Null Value Handling Examples ===")
	nullValueHandling()
}

// Basic encoding using json.Encode()
func basicEncoding() {
	fmt.Println("Basic value encoding using json.Encode():")

	// Encode basic types
	fmt.Println("\n1. Basic Types:")

	// String
	str := "Hello, 世界! <script>alert('xss')</script>"
	jsonStr, err := json.Encode(str)
	if err != nil {
		log.Printf("Error encoding string: %v", err)
	} else {
		fmt.Printf("String: %s -> %s\n", str, jsonStr)
		// Output: "Hello, 世界! \u003cscript\u003ealert('xss')\u003c/script\u003e"
	}

	// Integer
	num := 42
	jsonNum, err := json.Encode(num)
	if err != nil {
		log.Printf("Error encoding number: %v", err)
	} else {
		fmt.Printf("Integer: %d -> %s\n", num, jsonNum)
		// Output: 42
	}

	// Float
	pi := 3.14159265359
	jsonPi, err := json.Encode(pi)
	if err != nil {
		log.Printf("Error encoding float: %v", err)
	} else {
		fmt.Printf("Float: %f -> %s\n", pi, jsonPi)
		// Output: 3.14159265359
	}

	// Boolean
	active := true
	jsonBool, err := json.Encode(active)
	if err != nil {
		log.Printf("Error encoding boolean: %v", err)
	} else {
		fmt.Printf("Boolean: %t -> %s\n", active, jsonBool)
		// Output: true
	}

	// Nil value
	var nilValue *string = nil
	jsonNil, err := json.Encode(nilValue)
	if err != nil {
		log.Printf("Error encoding nil: %v", err)
	} else {
		fmt.Printf("Nil: %v -> %s\n", nilValue, jsonNil)
		// Output: null
	}

	// Array/Slice
	fmt.Println("\n2. Arrays and Slices:")
	numbers := []int{1, 2, 3, 4, 5}
	jsonArray, err := json.Encode(numbers)
	if err != nil {
		log.Printf("Error encoding array: %v", err)
	} else {
		fmt.Printf("Array: %v -> %s\n", numbers, jsonArray)
		// Output: [1, 2, 3, 4, 5]
	}

	// String slice
	tags := []string{"golang", "json", "encoding", "api"}
	jsonTags, err := json.Encode(tags)
	if err != nil {
		log.Printf("Error encoding string slice: %v", err)
	} else {
		fmt.Printf("String slice: %v -> %s\n", tags, jsonTags)
		// Output: ["golang", "json", "encoding", "api"]
	}

	// Map
	fmt.Println("\n3. Maps:")
	userMap := map[string]any{
		"name":   "John Doe",
		"age":    30,
		"active": true,
		"salary": 75000.50,
		"tags":   []string{"developer", "golang"},
	}
	jsonMap, err := json.Encode(userMap)
	if err != nil {
		log.Printf("Error encoding map: %v", err)
	} else {
		fmt.Printf("Map: %v -> %s\n", userMap, jsonMap)
		// Output: {"name": "John Doe", "age": 30, "active": true, "salary": 75000.5, "tags": ["developer", "golang"]}
	}

	// Simple struct
	fmt.Println("\n4. Simple Struct:")
	user := User{
		ID:       1,
		Name:     "Alice Johnson",
		Email:    "alice@example.com",
		Age:      28,
		Active:   true,
		Salary:   85000.75,
		Tags:     []string{"senior", "backend"},
		JoinedAt: time.Date(2022, 1, 15, 9, 0, 0, 0, time.UTC),
	}
	jsonUser, err := json.Encode(user)
	if err != nil {
		log.Printf("Error encoding struct: %v", err)
	} else {
		fmt.Printf("Struct: %s\n", jsonUser)
		// Output: {"id": 1, "name": "Alice Johnson", "email": "alice@example.com", "age": 28, "active": true, "salary": 85000.75, "tags": ["senior", "backend"], "joined_at": "2022-01-15T09:00:00Z"}
	}
}

// Pretty printing using json.EncodePretty()
func prettyPrinting() {
	fmt.Println("Pretty printing using json.EncodePretty():")

	// Create sample data
	user := User{
		ID:     1,
		Name:   "Bob Wilson",
		Email:  "bob@example.com",
		Age:    35,
		Active: true,
		Salary: 95000.00,
		Tags:   []string{"lead", "architect", "golang"},
		Metadata: &Metadata{
			Department: "Engineering",
			Level:      "Senior",
			Skills:     []string{"Go", "Docker", "Kubernetes", "PostgreSQL"},
			Projects: []Project{
				{
					Name:        "API Gateway",
					Status:      "active",
					Priority:    1,
					StartDate:   time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
					Budget:      50000.00,
					TeamMembers: []string{"Alice", "Charlie", "David"},
				},
				{
					Name:        "Microservices Migration",
					Status:      "completed",
					Priority:    2,
					StartDate:   time.Date(2023, 6, 1, 0, 0, 0, 0, time.UTC),
					EndDate:     &[]time.Time{time.Date(2023, 12, 31, 0, 0, 0, 0, time.UTC)}[0],
					Budget:      75000.00,
					TeamMembers: []string{"Bob", "Eve", "Frank"},
				},
			},
			Settings: map[string]any{
				"notifications": true,
				"theme":         "dark",
				"language":      "en",
				"timezone":      "UTC",
			},
			Attributes: map[string]string{
				"office":    "New York",
				"team":      "Backend",
				"clearance": "L2",
			},
		},
		JoinedAt: time.Date(2020, 3, 15, 10, 30, 0, 0, time.UTC),
	}

	// Default pretty printing
	fmt.Println("\n1. Default Pretty Printing:")
	prettyJSON, err := json.EncodePretty(user)
	if err != nil {
		log.Printf("Error in pretty encoding: %v", err)
	} else {
		fmt.Println("Pretty JSON:")
		fmt.Println(prettyJSON)
	}

	// Pretty printing with custom configuration
	fmt.Println("\n2. Pretty Printing with Custom Configuration:")
	config := json.NewPrettyConfig()
	config.Indent = "    " // 4 spaces instead of 2
	config.SortKeys = true // Sort keys alphabetically

	customPrettyJSON, err := json.EncodePretty(user, config)
	if err != nil {
		log.Printf("Error in custom pretty encoding: %v", err)
	} else {
		fmt.Println("Custom Pretty JSON (4-space indent, sorted keys):")
		fmt.Println(customPrettyJSON)
	}

	// Pretty printing with prefix
	fmt.Println("\n3. Pretty Printing with Prefix:")
	configWithPrefix := json.NewPrettyConfig()
	configWithPrefix.Prefix = " - "
	configWithPrefix.Indent = ".. "

	prefixPrettyJSON, err := json.EncodePretty(user, configWithPrefix)
	if err != nil {
		log.Printf("Error in prefix pretty encoding: %v", err)
	} else {
		fmt.Println("Pretty JSON with prefix:")
		fmt.Println(prefixPrettyJSON)
	}
}

// Compact encoding using json.EncodeCompact()
func compactEncoding() {
	fmt.Println("Compact encoding using json.EncodeCompact():")

	// Sample data with nested structure
	data := map[string]any{
		"users": []map[string]any{
			{
				"id":     1,
				"name":   "John Doe",
				"email":  "john@example.com",
				"active": true,
				"metadata": map[string]any{
					"department": "Engineering",
					"skills":     []string{"Go", "Python", "Docker"},
					"projects":   []string{"API", "Database", "Frontend"},
				},
			},
			{
				"id":     2,
				"name":   "Jane Smith",
				"email":  "jane@example.com",
				"active": false,
				"metadata": map[string]any{
					"department": "Marketing",
					"skills":     []string{"SEO", "Content", "Analytics"},
					"projects":   []string{"Campaign", "Website"},
				},
			},
		},
		"metadata": map[string]any{
			"total":   2,
			"version": "1.0",
			"created": time.Now().Format(time.RFC3339),
		},
	}

	// Default compact encoding
	fmt.Println("\n1. Default Compact Encoding:")
	compactJSON, err := json.EncodeCompact(data)
	if err != nil {
		log.Printf("Error in compact encoding: %v", err)
	} else {
		fmt.Printf("Compact JSON: %s\n", compactJSON)
		fmt.Printf("Length: %d characters\n", len(compactJSON))
	}

	// Compare with pretty printing
	fmt.Println("\n2. Comparison with Pretty Printing:")
	prettyJSON, err := json.EncodePretty(data)
	if err != nil {
		log.Printf("Error in pretty encoding: %v", err)
	} else {
		fmt.Printf("Pretty JSON length: %d characters\n", len(prettyJSON))
		fmt.Printf("Compact JSON length: %d characters\n", len(compactJSON))
		fmt.Printf("Size reduction: %.1f%%\n", float64(len(prettyJSON)-len(compactJSON))/float64(len(prettyJSON))*100)
	}

	// Compact encoding with custom configuration
	fmt.Println("\n3. Compact Encoding with Custom Configuration:")
	config := json.NewCompactConfig()
	config.EscapeHTML = false // Don't escape HTML characters

	customCompactJSON, err := json.EncodeCompact(data, config)
	if err != nil {
		log.Printf("Error in custom compact encoding: %v", err)
	} else {
		fmt.Printf("Custom compact JSON: %s\n", customCompactJSON)
	}
}

// Custom configuration encoding
func customConfiguration() {
	fmt.Println("Custom configuration encoding:")

	// Sample data with special characters
	data := map[string]any{
		"message":     "Hello <world> & \"friends\"! 🌍",
		"html":        "<script>alert('test')</script>",
		"unicode":     "Unicode: αβγδε 中文 🚀",
		"numbers":     []any{42, 3.14159, 1.23e-4, 999999999999999},
		"special":     "Line1\nLine2\tTabbed",
		"url":         "https://example.com/path?param=value",
		"empty_field": "",
		"null_field":  nil,
	}

	// 1. Custom configuration with HTML escaping disabled
	fmt.Println("\n1. HTML Escaping Disabled:")
	config1 := &json.EncodeConfig{
		Pretty:     true,
		Indent:     "  ",
		EscapeHTML: false,
		SortKeys:   true,
		MaxDepth:   100,
	}

	result1, err := json.Encode(data, config1)
	if err != nil {
		log.Printf("Error with HTML escaping disabled: %v", err)
	} else {
		fmt.Println("Result:")
		fmt.Println(result1)
	}

	// 2. Custom configuration with Unicode escaping
	fmt.Println("\n2. Unicode Escaping Enabled:")
	config2 := &json.EncodeConfig{
		Pretty:        true,
		Indent:        "  ",
		EscapeHTML:    true,
		EscapeUnicode: true,
		SortKeys:      true,
		MaxDepth:      100,
	}

	result2, err := json.Encode(data, config2)
	if err != nil {
		log.Printf("Error with Unicode escaping: %v", err)
	} else {
		fmt.Println("Result:")
		fmt.Println(result2)
	}

	// 3. Custom configuration with slash escaping
	fmt.Println("\n3. Slash Escaping Enabled:")
	config3 := &json.EncodeConfig{
		Pretty:      true,
		Indent:      "  ",
		EscapeHTML:  true,
		EscapeSlash: true,
		SortKeys:    true,
		MaxDepth:    100,
	}

	result3, err := json.Encode(data, config3)
	if err != nil {
		log.Printf("Error with slash escaping: %v", err)
	} else {
		fmt.Println("Result:")
		fmt.Println(result3)
	}

	// 4. Custom configuration with omit empty
	fmt.Println("\n4. Omit Empty Fields:")
	config4 := &json.EncodeConfig{
		Pretty:    true,
		Indent:    "  ",
		OmitEmpty: true,
		SortKeys:  true,
		MaxDepth:  100,
	}

	result4, err := json.Encode(data, config4)
	if err != nil {
		log.Printf("Error with omit empty: %v", err)
	} else {
		fmt.Println("Result:")
		fmt.Println(result4)
	}

	// 5. Custom configuration with number precision
	fmt.Println("\n5. Custom Float Precision:")
	config5 := &json.EncodeConfig{
		Pretty:         true,
		Indent:         "  ",
		FloatPrecision: 2, // 2 decimal places
		SortKeys:       true,
		MaxDepth:       100,
	}

	result5, err := json.Encode(data, config5)
	if err != nil {
		log.Printf("Error with float precision: %v", err)
	} else {
		fmt.Println("Result:")
		fmt.Println(result5)
	}
}

// Predefined configurations
func predefinedConfigurations() {
	fmt.Println("Predefined configurations:")

	// Sample data
	data := map[string]any{
		"name":         "Test User",
		"html_content": "<div>Hello <b>World</b>!</div>",
		"unicode":      "Unicode: 中文 🌟",
		"url":          "https://api.example.com/users/123",
		"script":       "<script>alert('xss')</script>",
		"numbers":      []float64{3.14159, 2.71828, 1.41421},
		"active":       true,
		"metadata": map[string]any{
			"created": time.Now().Format(time.RFC3339),
			"version": "1.0.0",
		},
	}

	// 1. Default configuration
	fmt.Println("\n1. Default Configuration:")
	defaultJSON, err := json.Encode(data)
	if err != nil {
		log.Printf("Error with default config: %v", err)
	} else {
		fmt.Printf("Default: %s\n", defaultJSON)
	}

	// 2. Pretty configuration
	fmt.Println("\n2. Pretty Configuration:")
	prettyJSON, err := json.Encode(data, json.NewPrettyConfig())
	if err != nil {
		log.Printf("Error with pretty config: %v", err)
	} else {
		fmt.Println("Pretty:")
		fmt.Println(prettyJSON)
	}

	// 3. Compact configuration
	fmt.Println("\n3. Compact Configuration:")
	compactJSON, err := json.Encode(data, json.NewCompactConfig())
	if err != nil {
		log.Printf("Error with compact config: %v", err)
	} else {
		fmt.Printf("Compact: %s\n", compactJSON)
	}

	// 4. Web-safe configuration
	fmt.Println("\n4. Web-Safe Configuration:")
	webSafeJSON, err := json.Encode(data, json.NewWebSafeConfig())
	if err != nil {
		log.Printf("Error with web-safe config: %v", err)
	} else {
		fmt.Printf("Web-safe: %s\n", webSafeJSON)
	}

	// 5. Readable configuration
	fmt.Println("\n5. Readable Configuration:")
	readableJSON, err := json.Encode(data, json.NewReadableConfig())
	if err != nil {
		log.Printf("Error with readable config: %v", err)
	} else {
		fmt.Println("Readable:")
		fmt.Println(readableJSON)
	}

	// Compare sizes
	fmt.Println("\n6. Size Comparison:")
	fmt.Printf("Default length:   %d\n", len(defaultJSON))
	fmt.Printf("Pretty length:    %d\n", len(prettyJSON))
	fmt.Printf("Compact length:   %d\n", len(compactJSON))
	fmt.Printf("Web-safe length:  %d\n", len(webSafeJSON))
	fmt.Printf("Readable length:  %d\n", len(readableJSON))
}

// Advanced encoding options
func advancedEncodingOptions() {
	fmt.Println("Advanced encoding options:")

	// Sample data with various special cases
	data := map[string]any{
		"text_with_escapes": "Line1\nLine2\tTabbed\rCarriage\"Quote'Single",
		"unicode_text":      "Emoji: 🚀🌟💻 Chinese: 中文测试 Greek: αβγδε",
		"html_content":      "<div class=\"test\">Hello & goodbye</div>",
		"json_string":       `{"nested": "json", "value": 123}`,
		"url_with_slashes":  "https://example.com/api/v1/users/123",
		"numbers": map[string]any{
			"integer":    42,
			"float":      3.14159265359,
			"scientific": 1.23e-10,
			"large":      999999999999999,
		},
		"empty_values": map[string]any{
			"empty_string": "",
			"empty_array":  []any{},
			"empty_object": map[string]any{},
			"null_value":   nil,
		},
	}

	// 1. Disable all escaping (except quotes and backslashes)
	fmt.Println("\n1. Minimal Escaping (DisableEscaping: true):")
	config1 := &json.EncodeConfig{
		Pretty:          true,
		Indent:          "  ",
		DisableEscaping: true,
		SortKeys:        true,
		MaxDepth:        100,
	}

	result1, err := json.Encode(data, config1)
	if err != nil {
		log.Printf("Error with minimal escaping: %v", err)
	} else {
		fmt.Println("Result:")
		fmt.Println(result1)
	}

	// 2. Custom escape characters
	fmt.Println("\n2. Custom Escape Characters:")
	config2 := &json.EncodeConfig{
		Pretty:   true,
		Indent:   "  ",
		SortKeys: true,
		MaxDepth: 100,
		CustomEscapes: map[rune]string{
			'🚀': "\\u{rocket}",
			'🌟': "\\u{star}",
			'💻': "\\u{computer}",
		},
	}

	result2, err := json.Encode(data, config2)
	if err != nil {
		log.Printf("Error with custom escapes: %v", err)
	} else {
		fmt.Println("Result:")
		fmt.Println(result2)
	}

	// 3. Advanced newline and tab handling
	fmt.Println("\n3. Custom Newline and Tab Handling:")
	config3 := &json.EncodeConfig{
		Pretty:         true,
		Indent:         "  ",
		EscapeNewlines: false, // Keep literal newlines
		EscapeTabs:     false, // Keep literal tabs
		SortKeys:       true,
		MaxDepth:       100,
	}

	result3, err := json.Encode(data, config3)
	if err != nil {
		log.Printf("Error with newline/tab handling: %v", err)
	} else {
		fmt.Println("Result:")
		fmt.Println(result3)
	}

	// 4. Maximum depth control
	fmt.Println("\n4. Maximum Depth Control:")
	deepData := map[string]any{
		"level1": map[string]any{
			"level2": map[string]any{
				"level3": map[string]any{
					"level4": map[string]any{
						"level5": "too deep",
					},
				},
			},
		},
	}

	config4 := &json.EncodeConfig{
		Pretty:   true,
		Indent:   "  ",
		MaxDepth: 3, // Limit to 3 levels
		SortKeys: true,
	}

	result4, err := json.Encode(deepData, config4)
	if err != nil {
		fmt.Printf("Expected error with max depth: %v\n", err)
	} else {
		fmt.Println("Result (should not reach here):")
		fmt.Println(result4)
	}

	// 5. Number preservation and precision
	fmt.Println("\n5. Number Preservation and Precision:")
	numberData := map[string]any{
		"precise_float":    3.141592653589793,
		"large_integer":    9007199254740991, // JavaScript safe integer limit
		"small_scientific": 1.23456789e-15,
		"currency":         99.99,
	}

	config5 := &json.EncodeConfig{
		Pretty:          true,
		Indent:          "  ",
		PreserveNumbers: true,
		FloatPrecision:  6, // 6 decimal places
		SortKeys:        true,
		MaxDepth:        100,
	}

	result5, err := json.Encode(numberData, config5)
	if err != nil {
		log.Printf("Error with number precision: %v", err)
	} else {
		fmt.Println("Result:")
		fmt.Println(result5)
	}
}

// Struct encoding with JSON tags
func structEncodingWithTags() {
	fmt.Println("Struct encoding with JSON tags:")

	// Create sample user with all fields
	user := User{
		ID:     123,
		Name:   "Emma Thompson",
		Email:  "emma@example.com",
		Age:    32,
		Active: true,
		Salary: 120000.50,
		Tags:   []string{"senior", "fullstack", "team-lead"},
		Metadata: &Metadata{
			Department: "Product Engineering",
			Level:      "Staff",
			Skills:     []string{"Go", "React", "PostgreSQL", "AWS", "Kubernetes"},
			Projects: []Project{
				{
					Name:        "User Dashboard",
					Status:      "active",
					Priority:    1,
					StartDate:   time.Date(2024, 2, 1, 0, 0, 0, 0, time.UTC),
					Budget:      25000.00,
					TeamMembers: []string{"Emma", "John", "Sarah"},
				},
			},
			Settings: map[string]any{
				"email_notifications": true,
				"slack_integration":   true,
				"dark_mode":           false,
				"auto_save":           true,
			},
			Attributes: map[string]string{
				"location":   "San Francisco",
				"timezone":   "PST",
				"manager_id": "456",
			},
		},
		JoinedAt: time.Date(2021, 6, 15, 9, 0, 0, 0, time.UTC),
	}

	// 1. Default struct encoding
	fmt.Println("\n1. Default Struct Encoding:")
	defaultJSON, err := json.EncodePretty(user)
	if err != nil {
		log.Printf("Error encoding struct: %v", err)
	} else {
		fmt.Println("Default encoding:")
		fmt.Println(defaultJSON)
	}

	// 2. Struct encoding with omit empty (some fields will be omitted if empty)
	fmt.Println("\n2. Struct with Empty Fields:")
	emptyUser := User{
		ID:       456,
		Name:     "Test User",
		Email:    "test@example.com",
		Age:      0, // This will show as 0, not omitted (int zero value)
		Active:   false,
		Salary:   0.0,
		Tags:     nil,         // This will be omitted due to omitempty tag
		Metadata: nil,         // This will be omitted due to omitempty tag
		JoinedAt: time.Time{}, // Zero time value
	}

	emptyJSON, err := json.EncodePretty(emptyUser)
	if err != nil {
		log.Printf("Error encoding empty struct: %v", err)
	} else {
		fmt.Println("Struct with empty fields:")
		fmt.Println(emptyJSON)
	}

	// 3. Struct encoding with custom configuration
	fmt.Println("\n3. Struct Encoding with Custom Configuration:")
	config := &json.EncodeConfig{
		Pretty:    true,
		Indent:    "    ", // 4 spaces
		SortKeys:  true,   // Sort JSON keys alphabetically
		OmitEmpty: false,  // Don't omit empty fields globally
		MaxDepth:  100,
	}

	customJSON, err := json.Encode(user, config)
	if err != nil {
		log.Printf("Error with custom config: %v", err)
	} else {
		fmt.Println("Custom configuration (4-space indent, sorted keys):")
		fmt.Println(customJSON)
	}

	// 4. Array of structs
	fmt.Println("\n4. Array of Structs:")
	users := []User{
		{
			ID:       1,
			Name:     "Alice Johnson",
			Email:    "alice@example.com",
			Age:      28,
			Active:   true,
			Salary:   85000.00,
			JoinedAt: time.Date(2022, 1, 15, 0, 0, 0, 0, time.UTC),
		},
		{
			ID:       2,
			Name:     "Bob Smith",
			Email:    "bob@example.com",
			Age:      35,
			Active:   false,
			Salary:   95000.00,
			JoinedAt: time.Date(2020, 8, 20, 0, 0, 0, 0, time.UTC),
		},
	}

	usersJSON, err := json.EncodePretty(users)
	if err != nil {
		log.Printf("Error encoding user array: %v", err)
	} else {
		fmt.Println("Array of users:")
		fmt.Println(usersJSON)
	}
}

// Complex data structure encoding
func complexDataStructureEncoding() {
	fmt.Println("Complex data structure encoding:")

	// Create a complex company structure
	revenue := 50000000.0
	company := Company{
		Name:    "TechCorp Solutions",
		Founded: 2015,
		Revenue: &revenue,
		Config: Config{
			Debug:       true,
			CacheTTL:    3600,
			Environment: "production",
			Features: map[string]bool{
				"auth":        true,
				"logging":     true,
				"metrics":     true,
				"rate_limit":  false,
				"maintenance": false,
			},
			Limits: map[string]int{
				"max_users":       10000,
				"max_requests":    1000000,
				"max_file_size":   104857600, // 100MB
				"session_timeout": 7200,      // 2 hours
			},
			Secrets: map[string]string{
				"api_key":     "***hidden***",
				"db_password": "***hidden***",
			},
		},
		Departments: []Department{
			{
				Name:   "Engineering",
				Budget: 2000000.0,
				Manager: &User{
					ID:       100,
					Name:     "Sarah Connor",
					Email:    "sarah@techcorp.com",
					Age:      42,
					Active:   true,
					Salary:   180000.0,
					JoinedAt: time.Date(2018, 3, 1, 0, 0, 0, 0, time.UTC),
				},
				Employees: []User{
					{
						ID:       101,
						Name:     "John Doe",
						Email:    "john@techcorp.com",
						Age:      30,
						Active:   true,
						Salary:   120000.0,
						Tags:     []string{"backend", "golang", "senior"},
						JoinedAt: time.Date(2020, 1, 15, 0, 0, 0, 0, time.UTC),
					},
					{
						ID:       102,
						Name:     "Jane Smith",
						Email:    "jane@techcorp.com",
						Age:      28,
						Active:   true,
						Salary:   115000.0,
						Tags:     []string{"frontend", "react", "senior"},
						JoinedAt: time.Date(2021, 6, 1, 0, 0, 0, 0, time.UTC),
					},
				},
			},
			{
				Name:   "Marketing",
				Budget: 800000.0,
				Employees: []User{
					{
						ID:       201,
						Name:     "Mike Johnson",
						Email:    "mike@techcorp.com",
						Age:      35,
						Active:   true,
						Salary:   90000.0,
						Tags:     []string{"marketing", "seo", "content"},
						JoinedAt: time.Date(2019, 9, 15, 0, 0, 0, 0, time.UTC),
					},
				},
			},
		},
		Employees: []User{}, // Will be populated from departments
	}

	// Populate company employees from departments
	for _, dept := range company.Departments {
		if dept.Manager != nil {
			company.Employees = append(company.Employees, *dept.Manager)
		}
		company.Employees = append(company.Employees, dept.Employees...)
	}

	// 1. Default complex structure encoding
	fmt.Println("\n1. Default Complex Structure Encoding:")
	defaultJSON, err := json.EncodePretty(company)
	if err != nil {
		log.Printf("Error encoding complex structure: %v", err)
	} else {
		fmt.Println("Complex structure (first 500 characters):")
		if len(defaultJSON) > 500 {
			fmt.Printf("%s...\n[truncated - total length: %d characters]\n", defaultJSON[:500], len(defaultJSON))
		} else {
			fmt.Println(defaultJSON)
		}
	}

	// 2. Compact encoding for comparison
	fmt.Println("\n2. Compact Encoding:")
	compactJSON, err := json.EncodeCompact(company)
	if err != nil {
		log.Printf("Error in compact encoding: %v", err)
	} else {
		fmt.Printf("Compact length: %d characters\n", len(compactJSON))
		fmt.Printf("Pretty length: %d characters\n", len(defaultJSON))
		fmt.Printf("Size reduction: %.1f%%\n", float64(len(defaultJSON)-len(compactJSON))/float64(len(defaultJSON))*100)
	}

	// 3. Selective field encoding (simulate partial data)
	fmt.Println("\n3. Selective Field Encoding:")
	summary := map[string]any{
		"company_name":      company.Name,
		"founded":           company.Founded,
		"total_employees":   len(company.Employees),
		"total_departments": len(company.Departments),
		"revenue":           company.Revenue,
		"department_summary": func() []map[string]any {
			var summary []map[string]any
			for _, dept := range company.Departments {
				summary = append(summary, map[string]any{
					"name":           dept.Name,
					"budget":         dept.Budget,
					"employee_count": len(dept.Employees),
					"has_manager":    dept.Manager != nil,
				})
			}
			return summary
		}(),
	}

	summaryJSON, err := json.EncodePretty(summary)
	if err != nil {
		log.Printf("Error encoding summary: %v", err)
	} else {
		fmt.Println("Company summary:")
		fmt.Println(summaryJSON)
	}
}

// Custom type encoding
func customTypeEncoding() {
	fmt.Println("Custom type encoding:")

	// 1. Custom type with MarshalJSON method
	fmt.Println("\n1. Custom Type with MarshalJSON:")
	products := []ProductWithCustomID{
		{
			ID:          CustomID{Prefix: "PROD", Number: 1},
			Name:        "Laptop Computer",
			Price:       1299.99,
			InStock:     true,
			Description: stringPtr("High-performance laptop for developers"),
		},
		{
			ID:      CustomID{Prefix: "PROD", Number: 2},
			Name:    "Wireless Mouse",
			Price:   29.99,
			InStock: false,
		},
		{
			ID:          CustomID{Prefix: "PROD", Number: 3},
			Name:        "Mechanical Keyboard",
			Price:       149.99,
			InStock:     true,
			Description: stringPtr("RGB backlit mechanical keyboard"),
		},
	}

	productsJSON, err := json.EncodePretty(products)
	if err != nil {
		log.Printf("Error encoding products: %v", err)
	} else {
		fmt.Println("Products with custom ID:")
		fmt.Println(productsJSON)
	}

	// 2. Interface{} with mixed types
	fmt.Println("\n2. Mixed Types in Interface{}:")
	mixedData := []any{
		"string value",
		42,
		3.14159,
		true,
		nil,
		[]int{1, 2, 3, 4, 5},
		map[string]any{
			"nested": "object",
			"count":  10,
		},
		CustomID{Prefix: "MIX", Number: 999},
		time.Now(),
	}

	mixedJSON, err := json.EncodePretty(mixedData)
	if err != nil {
		log.Printf("Error encoding mixed data: %v", err)
	} else {
		fmt.Println("Mixed types:")
		fmt.Println(mixedJSON)
	}

	// 3. Pointer handling
	fmt.Println("\n3. Pointer Handling:")
	type PointerExample struct {
		StringPtr *string `json:"string_ptr"`
		IntPtr    *int    `json:"int_ptr"`
		BoolPtr   *bool   `json:"bool_ptr"`
		StructPtr *User   `json:"struct_ptr,omitempty"`
		NilPtr    *string `json:"nil_ptr"`
		SlicePtr  *[]int  `json:"slice_ptr,omitempty"`
	}

	str := "pointer string"
	num := 42
	flag := true
	slice := []int{1, 2, 3}
	user := User{ID: 1, Name: "Pointer User", Email: "ptr@example.com", Age: 25, Active: true, Salary: 50000}

	pointerExample := PointerExample{
		StringPtr: &str,
		IntPtr:    &num,
		BoolPtr:   &flag,
		StructPtr: &user,
		NilPtr:    nil,
		SlicePtr:  &slice,
	}

	pointerJSON, err := json.EncodePretty(pointerExample)
	if err != nil {
		log.Printf("Error encoding pointers: %v", err)
	} else {
		fmt.Println("Pointer example:")
		fmt.Println(pointerJSON)
	}

	// 4. Channel, function, and c types (should cause errors)
	fmt.Println("\n4. Unsupported Types (should cause errors):")

	// Channel type
	ch := make(chan int)
	_, err = json.Encode(ch)
	if err != nil {
		fmt.Printf("Expected error for channel: %v\n", err)
	}

	// Function type
	fn := func() string { return "hello" }
	_, err = json.Encode(fn)
	if err != nil {
		fmt.Printf("Expected error for function: %v\n", err)
	}

	// Complex number
	c := complex(1, 2)
	_, err = json.Encode(c)
	if err != nil {
		fmt.Printf("Expected error for c: %v\n", err)
	}
}

// Helper function to create string pointer
func stringPtr(s string) *string {
	return &s
}

// ErrorMarshaler is a type that can cause marshaling errors
type ErrorMarshaler struct {
	Value string
}

// MarshalJSON implements custom JSON marshaling that can return an error
func (e ErrorMarshaler) MarshalJSON() ([]byte, error) {
	if e.Value == "error" {
		return nil, fmt.Errorf("custom marshaling error for value: %s", e.Value)
	}
	return []byte(fmt.Sprintf(`"%s"`, e.Value)), nil
}

// Performance optimization
func performanceOptimization() {
	fmt.Println("Performance optimization:")

	// Sample data for performance testing
	users := make([]User, 100)
	for i := 0; i < 100; i++ {
		users[i] = User{
			ID:       i + 1,
			Name:     fmt.Sprintf("User %d", i+1),
			Email:    fmt.Sprintf("user%d@example.com", i+1),
			Age:      20 + (i % 50),
			Active:   i%2 == 0,
			Salary:   50000.0 + float64(i*1000),
			Tags:     []string{"tag1", "tag2", "tag3"},
			JoinedAt: time.Now().AddDate(0, -i, 0),
		}
	}

	// 1. Processor reuse vs creating new processors
	fmt.Println("\n1. Processor Reuse vs New Processors:")

	// Using default functions (creates new processor each time)
	start := time.Now()
	for i := 0; i < 10; i++ {
		_, err := json.Encode(users[i])
		if err != nil {
			log.Printf("Error with default function: %v", err)
		}
	}
	defaultDuration := time.Since(start)

	// Using reused processor
	processor := json.New()
	defer processor.Close()

	start = time.Now()
	for i := 0; i < 10; i++ {
		_, err := processor.EncodeWithConfig(users[i], json.DefaultEncodeConfig())
		if err != nil {
			log.Printf("Error with reused processor: %v", err)
		}
	}
	reusedDuration := time.Since(start)

	fmt.Printf("Default functions took: %v\n", defaultDuration)
	fmt.Printf("Reused processor took: %v\n", reusedDuration)
	if reusedDuration > 0 {
		fmt.Printf("Processor reuse is %.2fx faster\n", float64(defaultDuration.Nanoseconds())/float64(reusedDuration.Nanoseconds()))
	}

	// 2. Compact vs Pretty encoding performance
	fmt.Println("\n2. Compact vs Pretty Encoding Performance:")

	start = time.Now()
	compactResult, err := json.EncodeCompact(users)
	compactDuration := time.Since(start)
	if err != nil {
		log.Printf("Error in compact encoding: %v", err)
	}

	start = time.Now()
	prettyResult, err := json.EncodePretty(users)
	prettyDuration := time.Since(start)
	if err != nil {
		log.Printf("Error in pretty encoding: %v", err)
	}

	fmt.Printf("Compact encoding took: %v (result: %d chars)\n", compactDuration, len(compactResult))
	fmt.Printf("Pretty encoding took: %v (result: %d chars)\n", prettyDuration, len(prettyResult))
	if compactDuration > 0 {
		fmt.Printf("Compact is %.2fx faster\n", float64(prettyDuration.Nanoseconds())/float64(compactDuration.Nanoseconds()))
	}

	// 3. Configuration reuse
	fmt.Println("\n3. Configuration Reuse:")
	config := json.NewPrettyConfig()
	config.SortKeys = true

	start = time.Now()
	for i := 0; i < 5; i++ {
		_, err := json.Encode(users[i], config) // Reuse same config
		if err != nil {
			log.Printf("Error with config reuse: %v", err)
		}
	}
	configReuseDuration := time.Since(start)

	start = time.Now()
	for i := 0; i < 5; i++ {
		newConfig := json.NewPrettyConfig() // Create new config each time
		newConfig.SortKeys = true
		_, err := json.Encode(users[i], newConfig)
		if err != nil {
			log.Printf("Error with new config: %v", err)
		}
	}
	newConfigDuration := time.Since(start)

	fmt.Printf("Config reuse took: %v\n", configReuseDuration)
	fmt.Printf("New config each time took: %v\n", newConfigDuration)
	if configReuseDuration > 0 {
		fmt.Printf("Config reuse is %.2fx faster\n", float64(newConfigDuration.Nanoseconds())/float64(configReuseDuration.Nanoseconds()))
	}

	// 4. Memory usage comparison
	fmt.Println("\n4. Memory Usage Tips:")
	fmt.Println("- Reuse processors instead of creating new ones")
	fmt.Println("- Reuse configuration objects")
	fmt.Println("- Use compact encoding for network transmission")
	fmt.Println("- Use pretty encoding only for debugging/display")
	fmt.Println("- Consider streaming for very large datasets")
}

// Error handling examples
func errorHandlingExamples() {
	fmt.Println("Error handling examples:")

	// 1. Unsupported types
	fmt.Println("\n1. Unsupported Types:")

	// Channel
	ch := make(chan int)
	_, err := json.Encode(ch)
	if err != nil {
		fmt.Printf("Channel encoding error: %v\n", err)
	}

	// Function
	fn := func() {}
	_, err = json.Encode(fn)
	if err != nil {
		fmt.Printf("Function encoding error: %v\n", err)
	}

	// Complex number
	complex := complex(1, 2)
	_, err = json.Encode(complex)
	if err != nil {
		fmt.Printf("Complex number encoding error: %v\n", err)
	}

	// 2. Circular references
	fmt.Println("\n2. Circular References:")
	type Node struct {
		Name string `json:"name"`
		Next *Node  `json:"next,omitempty"`
	}

	node1 := &Node{Name: "Node1"}
	node2 := &Node{Name: "Node2"}
	node1.Next = node2
	node2.Next = node1 // Creates circular reference

	_, err = json.Encode(node1)
	if err != nil {
		fmt.Printf("Circular reference error: %v\n", err)
	}

	// 3. Maximum depth exceeded
	fmt.Println("\n3. Maximum Depth Exceeded:")
	deepData := make(map[string]any)
	current := deepData

	// Create deeply nested structure
	for i := 0; i < 10; i++ {
		next := make(map[string]any)
		current[fmt.Sprintf("level%d", i)] = next
		current = next
	}
	current["final"] = "value"

	config := &json.EncodeConfig{
		Pretty:   true,
		MaxDepth: 5, // Limit depth to 5
	}

	_, err = json.Encode(deepData, config)
	if err != nil {
		fmt.Printf("Max depth error: %v\n", err)
	}

	// 4. Invalid UTF-8 handling
	fmt.Println("\n4. Invalid UTF-8 Handling:")
	invalidUTF8 := map[string]any{
		"valid":   "Hello, 世界!",
		"invalid": string([]byte{0xff, 0xfe, 0xfd}), // Invalid UTF-8 sequence
	}

	configUTF8 := &json.EncodeConfig{
		ValidateUTF8: true,
		Pretty:       true,
		MaxDepth:     100,
	}

	result, err := json.Encode(invalidUTF8, configUTF8)
	if err != nil {
		fmt.Printf("UTF-8 validation error: %v\n", err)
	} else {
		fmt.Printf("UTF-8 result: %s\n", result)
	}

	// 5. Custom marshaler errors
	fmt.Println("\n5. Custom Marshaler Errors:")
	errorData := []ErrorMarshaler{
		{Value: "good"},
		{Value: "error"}, // This will cause an error
		{Value: "also_good"},
	}

	_, err = json.Encode(errorData)
	if err != nil {
		fmt.Printf("Custom marshaler error: %v\n", err)
	}

	// 6. Successful error recovery
	fmt.Println("\n6. Successful Error Recovery:")
	goodData := map[string]any{
		"name":    "Valid Data",
		"count":   42,
		"active":  true,
		"tags":    []string{"valid", "data", "example"},
		"created": time.Now(),
	}

	successResult, err := json.EncodePretty(goodData)
	if err != nil {
		fmt.Printf("Unexpected error: %v\n", err)
	} else {
		fmt.Println("Successful encoding after errors:")
		fmt.Println(successResult)
	}

	fmt.Println("\nError Handling Best Practices:")
	fmt.Println("- Always check for errors when encoding")
	fmt.Println("- Be aware of unsupported types (chan, func, complex)")
	fmt.Println("- Avoid circular references in data structures")
	fmt.Println("- Set appropriate MaxDepth limits for deeply nested data")
	fmt.Println("- Validate UTF-8 when dealing with external data")
	fmt.Println("- Handle custom marshaler errors gracefully")
}

var line = strings.Repeat("----", 20)

func printLines(title string) {
	fmt.Println(line)
	fmt.Println(title)
}

// nullValueHandling demonstrates the IncludeNulls configuration option
func nullValueHandling() {
	fmt.Println("Demonstrating null value handling with IncludeNulls configuration:")

	// Define a struct with nil values
	type UserProfile struct {
		Name     string          `json:"name"`
		Email    *string         `json:"email"`
		Phone    *string         `json:"phone"`
		Tags     []string        `json:"tags"`
		Metadata *map[string]any `json:"metadata"`
		Active   *bool           `json:"active"`
		Age      *int            `json:"age"`
	}

	// Create test data with nil values
	profile := UserProfile{
		Name:     "Alice Johnson",
		Email:    nil, // nil pointer
		Phone:    nil, // nil pointer
		Tags:     nil, // nil slice
		Metadata: nil, // nil pointer to map
		Active:   nil, // nil pointer to bool
		Age:      nil, // nil pointer to int
	}

	fmt.Println("\nTest data contains nil values for: Email, Phone, Tags, Metadata, Active, Age")

	// 1. Default behavior (IncludeNulls = true)
	fmt.Println("\n1. Default behavior (IncludeNulls = true):")
	fmt.Println("   All fields are included, nil values become 'null'")
	defaultResult, err := json.EncodePretty(profile)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	} else {
		fmt.Printf("Result:\n%s\n", defaultResult)
	}

	// 2. IncludeNulls = false
	fmt.Println("\n2. IncludeNulls = false:")
	fmt.Println("   Nil fields are omitted from output")
	cleanConfig := &json.EncodeConfig{
		Pretty:       true,
		Indent:       "  ",
		IncludeNulls: false,
		SortKeys:     true,
		MaxDepth:     100,
	}
	cleanResult, err := json.Encode(profile, cleanConfig)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	} else {
		fmt.Printf("Result:\n%s\n", cleanResult)
	}

	// 3. Using NewCleanConfig() predefined configuration
	fmt.Println("\n3. Using NewCleanConfig() (predefined clean configuration):")
	fmt.Println("   Convenient preset for clean output without null values")
	presetResult, err := json.Encode(profile, json.NewCleanConfig())
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	} else {
		fmt.Printf("Result:\n%s\n", presetResult)
	}

	fmt.Println("\nKey benefits:")
	fmt.Println("- IncludeNulls = true (default): Preserves data structure, shows all fields")
	fmt.Println("- IncludeNulls = false: Creates cleaner output, reduces JSON size")
	fmt.Println("- Works with both structs and maps")
	fmt.Println("- Combines well with Pretty formatting and SortKeys")
}
