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
// Run: go run examples/5_encoding_options.go

func main() {
	fmt.Println("⚙️  JSON Library - Encoding Options")
	fmt.Println("==================================\n ")

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
	fmt.Println("1️⃣  Pretty vs Compact Formatting")
	fmt.Println("─────────────────────────────────")
	demonstratePrettyVsCompact(user)

	// 2. HTML ESCAPING
	fmt.Println("\n2️⃣  HTML Escaping Control")
	fmt.Println("──────────────────────────")
	demonstrateHTMLEscaping()

	// 3. KEY SORTING
	fmt.Println("\n3️⃣  Key Sorting")
	fmt.Println("───────────────")
	demonstrateKeySorting()

	// 4. FLOAT PRECISION
	fmt.Println("\n4️⃣  Float Precision Control")
	fmt.Println("──────────────────────────")
	demonstrateFloatPrecision()

	// 5. OMIT EMPTY
	fmt.Println("\n5️⃣  Omit Empty Values")
	fmt.Println("──────────────────────")
	demonstrateOmitEmpty()

	// 6. CUSTOM ESCAPING
	fmt.Println("\n6️⃣  Custom Escaping Options")
	fmt.Println("───────────────────────────")
	demonstrateCustomEscaping()

	// 7. UNICODE ESCAPING
	fmt.Println("\n7️⃣  Unicode Escaping")
	fmt.Println("─────────────────────")
	demonstrateUnicodeEscaping()

	// 8. ENCODE METHODS
	fmt.Println("\n8️⃣  Convenience Encode Methods")
	fmt.Println("────────────────────────────────")
	demonstrateEncodeMethods()

	fmt.Println("\n✅ Encoding options complete!")
}

func demonstratePrettyVsCompact(user interface{}) {
	// Pretty formatting
	prettyJSON, _ := json.EncodePretty(user)
	fmt.Println("   Pretty JSON:")
	fmt.Println(prettyJSON)

	// Compact formatting
	compactJSON, _ := json.Encode(user)
	fmt.Println("\n   Compact JSON:")
	fmt.Println(compactJSON)
}

func demonstrateHTMLEscaping() {
	// Data with HTML content
	type HTMLContent struct {
		Title   string `json:"title"`
		Content string `json:"content"`
	}

	data := HTMLContent{
		Title:   "Hello <script>alert('XSS')</script>",
		Content: "Visit <a href='https://example.com'>here</a>",
	}

	// With HTML escaping (default, safe for web)
	configWithEscape := json.DefaultEncodeConfig()
	configWithEscape.EscapeHTML = true
	configWithEscape.Pretty = true

	escapedJSON, _ := json.Encode(data, configWithEscape)
	fmt.Println("   With HTML escaping (safe for web):")
	fmt.Println(escapedJSON)

	// Without HTML escaping
	configWithoutEscape := json.DefaultEncodeConfig()
	configWithoutEscape.EscapeHTML = false
	configWithoutEscape.Pretty = true

	unescapedJSON, _ := json.Encode(data, configWithoutEscape)
	fmt.Println("\n   Without HTML escaping (readable):")
	fmt.Println(unescapedJSON)
}

func demonstrateKeySorting() {
	type Data struct {
		Zebra   int `json:"zebra"`
		Alpha   int `json:"alpha"`
		Charlie int `json:"charlie"`
		Beta    int `json:"beta"`
	}

	data := Data{Zebra: 1, Alpha: 2, Charlie: 3, Beta: 4}

	// Without sorting (default insertion order)
	configUnsorted := json.DefaultEncodeConfig()
	configUnsorted.Pretty = true
	configUnsorted.SortKeys = false

	unsortedJSON, _ := json.Encode(data, configUnsorted)
	fmt.Println("   Without key sorting:")
	fmt.Println(unsortedJSON)

	// With sorting
	configSorted := json.DefaultEncodeConfig()
	configSorted.Pretty = true
	configSorted.SortKeys = true

	sortedJSON, _ := json.Encode(data, configSorted)
	fmt.Println("\n   With key sorting:")
	fmt.Println(sortedJSON)
}

func demonstrateFloatPrecision() {
	type Measurement struct {
		Name  string  `json:"name"`
		Value float64 `json:"value"`
	}

	data := Measurement{
		Name:  "pi",
		Value: 3.141592653589793,
	}

	// Default precision
	configDefault := json.DefaultEncodeConfig()
	configDefault.Pretty = true
	configDefault.FloatPrecision = -1 // Auto precision

	defaultJSON, _ := json.Encode(data, configDefault)
	fmt.Println("   Default precision (auto):")
	fmt.Println(defaultJSON)

	// Fixed precision: 2 decimal places
	configFixed2 := json.DefaultEncodeConfig()
	configFixed2.Pretty = true
	configFixed2.FloatPrecision = 2

	fixed2JSON, _ := json.Encode(data, configFixed2)
	fmt.Println("\n   Fixed precision (2 decimals):")
	fmt.Println(fixed2JSON)

	// Fixed precision: 4 decimal places
	configFixed4 := json.DefaultEncodeConfig()
	configFixed4.Pretty = true
	configFixed4.FloatPrecision = 4

	fixed4JSON, _ := json.Encode(data, configFixed4)
	fmt.Println("\n   Fixed precision (4 decimals):")
	fmt.Println(fixed4JSON)
}

func demonstrateOmitEmpty() {
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

	config := json.DefaultEncodeConfig()
	config.Pretty = true

	fullJSON, _ := json.Encode(fullConfig, config)
	fmt.Println("   Full config (all fields shown):")
	fmt.Println(fullJSON)

	minimalJSON, _ := json.Encode(minimalConfig, config)
	fmt.Println("\n   Minimal config (empty fields handled by tags):")
	fmt.Println("   - Password: omitted (has omitempty tag)")
	fmt.Println("   - Database: included (no omitempty tag)")
	fmt.Println(minimalJSON)
}

func demonstrateCustomEscaping() {
	// Data with special characters
	type Message struct {
		Text string `json:"text"`
	}

	data := Message{
		Text: "Line1\nLine2\tTabbed\r\nBackslash: \\",
	}

	// Default escaping (newlines and tabs escaped)
	configDefault := json.DefaultEncodeConfig()
	configDefault.EscapeNewlines = true
	configDefault.EscapeTabs = true
	configDefault.Pretty = true

	defaultJSON, _ := json.Encode(data, configDefault)
	fmt.Println("   With newline/tab escaping:")
	fmt.Println(defaultJSON)

	// Without newline/tab escaping
	configRaw := json.DefaultEncodeConfig()
	configRaw.EscapeNewlines = false
	configRaw.EscapeTabs = false
	configRaw.Pretty = true

	rawJSON, _ := json.Encode(data, configRaw)
	fmt.Println("\n   Without newline/tab escaping:")
	fmt.Println(rawJSON)

	// With slash escaping
	configSlash := json.DefaultEncodeConfig()
	configSlash.EscapeSlash = true
	configSlash.Pretty = true

	dataWithSlash := Message{Text: "https://example.com/path"}
	slashJSON, _ := json.Encode(dataWithSlash, configSlash)
	fmt.Println("\n   With slash escaping:")
	fmt.Println(slashJSON)
}

func demonstrateUnicodeEscaping() {
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
	configReadable := json.DefaultEncodeConfig()
	configReadable.EscapeUnicode = false
	configReadable.Pretty = true

	readableJSON, _ := json.Encode(data, configReadable)
	fmt.Println("   Unicode as-is (readable):")
	fmt.Println(readableJSON)

	// With Unicode escaping (ASCII safe)
	configEscaped := json.DefaultEncodeConfig()
	configEscaped.EscapeUnicode = true
	configEscaped.Pretty = true

	escapedJSON, _ := json.Encode(data, configEscaped)
	fmt.Println("\n   Unicode escaped (ASCII safe):")
	fmt.Println(escapedJSON)
}

func demonstrateEncodeMethods() {
	type Product struct {
		ID    int     `json:"id"`
		Name  string  `json:"name"`
		Price float64 `json:"price"`
	}

	product := Product{ID: 1, Name: "Laptop", Price: 999.99}

	// Encode (compact by default)
	compact, _ := json.Encode(product)
	fmt.Printf("   Encode (compact): %s\n", compact)

	// EncodePretty
	pretty, _ := json.EncodePretty(product)
	fmt.Println("\n   EncodePretty:")
	fmt.Println(pretty)

	// Encode with custom configuration
	customCfg := json.DefaultEncodeConfig()
	customCfg.Pretty = true
	customCfg.Indent = "    "
	custom, _ := json.Encode(product, customCfg)
	fmt.Println("\n   Encode with custom config (4-space indent):")
	fmt.Println(custom)
}
