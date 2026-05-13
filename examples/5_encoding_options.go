//go:build example

package main

import (
	"fmt"

	"github.com/cybergodev/json"
)

// Encoding Options Example
//
// This example demonstrates advanced JSON encoding options for customization.
// Learn about custom escaping, key sorting, number precision, and more.
//
// Topics covered:
// - Custom encoding with EncodeConfig
// - HTML escaping control
// - Key sorting for consistent output
// - Float precision control
// - Omit empty values
// - Custom escape sequences
// - Pretty vs compact formatting
//
// Run: go run -tags=example examples/5_encoding_options.go

func main() {
	fmt.Println("JSON Library - Encoding Options")
	fmt.Println("==================================")

	// Sample data
	type User struct {
		Name      string  `json:"name"`
		Age       int     `json:"age"`
		Score     float64 `json:"score"`
		Email     string  `json:"email,omitempty"`
		Bio       string  `json:"bio,omitempty"`
		Active    bool    `json:"active"`
		Hidden    string  `json:"-"` // Always omitted
		CreatedAt string  `json:"created_at"`
		Hobby     any     `json:"hobby"`
	}

	user := User{
		Name:      "John Doe",
		Age:       30,
		Score:     95.6789,
		Active:    true,
		CreatedAt: "2024-01-15T10:30:00Z",
		Hidden:    "secret",
		Hobby:     map[string]any{"name": "reading", "level": "advanced"},
	}

	// 1. PRETTY VS COMPACT
	demonstratePrettyVsCompact(user)

	// 2. HTML ESCAPING
	demonstrateHTMLEscaping()

	// 3. KEY SORTING
	demonstrateKeySorting()

	// 4. FLOAT PRECISION
	demonstrateFloatPrecision()

	// 5. OMIT EMPTY
	demonstrateOmitEmpty()

	// 6. CUSTOM ESCAPING
	demonstrateCustomEscaping()

	// 7. UNICODE ESCAPING
	demonstrateUnicodeEscaping()

	// 8. ENCODE METHODS
	demonstrateEncodeMethods()

	fmt.Println("\nEncoding options complete!")
}

func demonstratePrettyVsCompact(user any) {
	fmt.Println("1. Pretty vs Compact Formatting")
	fmt.Println("---------------------------------")

	// Pretty formatting - using unified Config API
	prettyJSON, err := json.EncodeWithConfig(user, json.PrettyConfig())
	if err != nil {
		fmt.Printf("   EncodeWithConfig error: %v\n", err)
		return
	}
	fmt.Println("   Pretty JSON:")
	fmt.Println(prettyJSON)

	// Compact formatting (default)
	compactJSON, err := json.Encode(user)
	if err != nil {
		fmt.Printf("   Encode error: %v\n", err)
		return
	}
	fmt.Println("\n   Compact JSON:")
	fmt.Println(compactJSON)
}

func demonstrateHTMLEscaping() {
	fmt.Println("\n2. HTML Escaping Control")
	fmt.Println("--------------------------")

	// Data with HTML content
	type HTMLContent struct {
		Title   string `json:"title"`
		Content string `json:"content"`
	}

	data := HTMLContent{
		Title:   "Hello <script>alert('XSS')</script>",
		Content: "Visit <a href='https://example.com'>here</a>",
	}

	// With HTML escaping + slash escaping (safe for web embedding)
	webSafeConfig := json.DefaultConfig()
	webSafeConfig.EscapeHTML = true
	webSafeConfig.EscapeSlash = true
	escapedJSON, _ := json.Encode(data, webSafeConfig)
	fmt.Println("   With HTML+slash escaping (safe for web):")
	fmt.Println(escapedJSON)

	// Without HTML escaping (for readability, not for web embedding)
	readableConfig := json.PrettyConfig()
	readableConfig.EscapeHTML = false
	readableConfig.DisableEscaping = true
	unescapedJSON, err := json.Encode(data, readableConfig)
	if err != nil {
		fmt.Printf("   Encode error: %v\n", err)
		return
	}
	fmt.Println("\n   Without HTML escaping (readable output):")
	fmt.Println(unescapedJSON)
}

func demonstrateKeySorting() {
	fmt.Println("\n3. Key Sorting")
	fmt.Println("---------------")

	type Data struct {
		Zebra   int `json:"zebra"`
		Alpha   int `json:"alpha"`
		Charlie int `json:"charlie"`
		Beta    int `json:"beta"`
	}

	data := Data{Zebra: 1, Alpha: 2, Charlie: 3, Beta: 4}

	// Without sorting (default insertion order)
	unsortedJSON, _ := json.EncodePretty(data)
	fmt.Println("   Without key sorting:")
	fmt.Println(unsortedJSON)

	// With sorting
	cfgSorted := json.PrettyConfig()
	cfgSorted.SortKeys = true

	sortedJSON, _ := json.Encode(data, cfgSorted)
	fmt.Println("\n   With key sorting:")
	fmt.Println(sortedJSON)
}

func demonstrateFloatPrecision() {
	fmt.Println("\n4. Float Precision Control")
	fmt.Println("--------------------------")

	type Measurement struct {
		Name  string  `json:"name"`
		Value float64 `json:"value"`
	}

	data := Measurement{
		Name:  "pi",
		Value: 3.141592653589793,
	}

	// Default precision
	defaultJSON, _ := json.EncodePretty(data)
	fmt.Println("   Default precision (auto):")
	fmt.Println(defaultJSON)

	// Fixed precision: 2 decimal places (rounding)
	cfg2 := json.PrettyConfig()
	cfg2.FloatPrecision = 2

	fixed2JSON, _ := json.Encode(data, cfg2)
	fmt.Println("\n   Fixed precision (2 decimals, rounded):")
	fmt.Println(fixed2JSON)

	// Fixed precision: 4 decimal places (rounding)
	cfg4 := json.PrettyConfig()
	cfg4.FloatPrecision = 4

	fixed4JSON, _ := json.Encode(data, cfg4)
	fmt.Println("\n   Fixed precision (4 decimals, rounded):")
	fmt.Println(fixed4JSON)

	// Fixed precision: 4 decimal places (truncate)
	cfgTrunc := json.PrettyConfig()
	cfgTrunc.FloatPrecision = 4
	cfgTrunc.FloatTruncate = true // Enable truncation

	truncateJSON, _ := json.Encode(data, cfgTrunc)
	fmt.Println("\n   Fixed precision (4 decimals, truncated):")
	fmt.Println(truncateJSON)
}

func demonstrateOmitEmpty() {
	fmt.Println("\n5. Omit Empty Values")
	fmt.Println("----------------------")

	type Config struct {
		Host     string `json:"host"`
		Port     int    `json:"port"`
		Username string `json:"username,omitempty"`
		Password string `json:"password,omitempty"`
		Database string `json:"database"` // No omitempty tag
	}

	// Full config - all fields have values
	fullConfig := Config{
		Host:     "localhost",
		Port:     5432,
		Username: "admin",
		Password: "secret123",
		Database: "mydb",
	}

	// Minimal config - some fields are empty
	minimalConfig := Config{
		Host:     "localhost",
		Port:     5432,
		Username: "admin",
		Password: "", // Empty, will be omitted due to omitempty tag
		Database: "", // Empty, but no tag so will be included
	}

	fullJSON, _ := json.EncodePretty(fullConfig)
	fmt.Println("   Full config (all fields shown):")
	fmt.Println(fullJSON)

	minimalJSON, _ := json.EncodePretty(minimalConfig)
	fmt.Println("\n   Minimal config (empty fields handled by tags):")
	fmt.Println("   - Password: omitted (has omitempty tag)")
	fmt.Println("   - Database: included (no omitempty tag)")
	fmt.Println(minimalJSON)
}

func demonstrateCustomEscaping() {
	fmt.Println("\n6. Custom Escaping Options")
	fmt.Println("---------------------------")

	// Data with special characters
	type Message struct {
		Text string `json:"text"`
	}

	data := Message{
		Text: "Line1\nLine2\tTabbed\r\nBackslash: \\",
	}

	// Default escaping (newlines and tabs escaped)
	cfgDefault := json.PrettyConfig()
	cfgDefault.EscapeNewlines = true
	cfgDefault.EscapeTabs = true

	defaultJSON, _ := json.Encode(data, cfgDefault)
	fmt.Println("   With newline/tab escaping:")
	fmt.Println(defaultJSON)

	// Without newline/tab escaping
	cfgRaw := json.PrettyConfig()
	cfgRaw.EscapeNewlines = false
	cfgRaw.EscapeTabs = false

	rawJSON, _ := json.Encode(data, cfgRaw)
	fmt.Println("\n   Without newline/tab escaping:")
	fmt.Println(rawJSON)

	// With slash escaping
	cfgSlash := json.PrettyConfig()
	cfgSlash.EscapeSlash = true

	dataWithSlash := Message{Text: "https://example.com/path"}
	slashJSON, _ := json.Encode(dataWithSlash, cfgSlash)
	fmt.Println("\n   With slash escaping:")
	fmt.Println(slashJSON)
}

func demonstrateUnicodeEscaping() {
	fmt.Println("\n7. Unicode Escaping")
	fmt.Println("---------------------")

	// Data with Unicode characters
	type Greeting struct {
		Emoji   string `json:"emoji"`
		Chinese string `json:"chinese"`
		Arabic  string `json:"arabic"`
		Symbol  string `json:"symbol"`
	}

	data := Greeting{
		Emoji:   "Hello 🌍🚀",
		Chinese: "你好世界",
		Arabic:  "مرحبا",
		Symbol:  "© 2024 ★",
	}

	// Without Unicode escaping (readable)
	readableJSON, _ := json.EncodePretty(data)
	fmt.Println("   Unicode as-is (readable):")
	fmt.Println(readableJSON)

	// With Unicode escaping (ASCII safe)
	cfgEscaped := json.PrettyConfig()
	cfgEscaped.EscapeUnicode = true

	escapedJSON, _ := json.Encode(data, cfgEscaped)
	fmt.Println("\n   Unicode escaped (ASCII safe):")
	fmt.Println(escapedJSON)
}

func demonstrateEncodeMethods() {
	fmt.Println("\n8. Convenience Encode Methods")
	fmt.Println("--------------------------------")

	type Product struct {
		ID    int     `json:"id"`
		Name  string  `json:"name"`
		Price float64 `json:"price"`
	}

	product := Product{ID: 1, Name: "Laptop", Price: 999.99}

	// Encode (compact by default)
	compact, err := json.Encode(product)
	if err != nil {
		fmt.Printf("   Encode error: %v\n", err)
		return
	}
	fmt.Printf("   Encode (compact): %s\n", compact)

	// EncodePretty (convenience for pretty output)
	pretty, err := json.EncodePretty(product)
	if err != nil {
		fmt.Printf("   EncodePretty error: %v\n", err)
		return
	}
	fmt.Println("\n   EncodePretty (pretty, zero-config):")
	fmt.Println(pretty)

	// EncodeWithConfig (pretty with custom options)
	prettyCfg, err := json.EncodeWithConfig(product, json.PrettyConfig())
	if err != nil {
		fmt.Printf("   EncodeWithConfig error: %v\n", err)
		return
	}
	fmt.Println("\n   EncodeWithConfig (pretty with custom options):")
	fmt.Println(prettyCfg)

	// Encode with custom configuration
	customCfg := json.PrettyConfig()
	customCfg.Indent = "    "
	custom, err := json.Encode(product, customCfg)
	if err != nil {
		fmt.Printf("   Encode error: %v\n", err)
		return
	}
	fmt.Println("\n   Encode with custom config (4-space indent):")
	fmt.Println(custom)
}
