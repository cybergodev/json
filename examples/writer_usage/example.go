package main

import (
	"bytes"
	"fmt"
	"log"
	"os"

	"github.com/cybergodev/json"
)

func main() {
	fmt.Println("🚀 Correct Usage of SaveToWriter and SaveToFile Examples")
	fmt.Println("==========================================")

	// Sample data
	userData := map[string]any{
		"user": map[string]any{
			"id":   1001,
			"name": "Alice",
			"age":  30,
			"profile": map[string]any{
				"email": "alice@example.com",
				"phone": "+1-555-0123",
			},
		},
		"settings": map[string]any{
			"theme":         "dark",
			"notifications": true,
			"language":      "en",
		},
	}

	processor := json.New()
	defer processor.Close()

	fmt.Println("\n1️⃣ Correct Usage of SaveToWriter")
	fmt.Println("==============================")

	// ✅ Correct Method 1: Save data object directly to Writer
	var buffer1 bytes.Buffer
	err := processor.SaveToWriter(&buffer1, userData, false) // Compact format
	if err != nil {
		log.Printf("SaveToWriter failed: %v", err)
		return
	}
	fmt.Printf("✅ Method 1 - Save data object directly, Buffer size: %d bytes\n", buffer1.Len())

	// ✅ Correct Method 2: Save formatted data object to Writer
	var buffer2 bytes.Buffer
	err = processor.SaveToWriter(&buffer2, userData, true) // Formatted
	if err != nil {
		log.Printf("SaveToWriter failed: %v", err)
		return
	}
	fmt.Printf("✅ Method 2 - Save formatted data object, Buffer size: %d bytes\n", buffer2.Len())

	fmt.Println("\n2️⃣ Correct Usage of SaveToFile")
	fmt.Println("=============================")

	// ✅ Correct Method 1: Save data object directly to file
	err = json.SaveToFile("dev_test/user_data_compact.json", userData, false)
	if err != nil {
		log.Printf("SaveToFile failed: %v", err)
	} else {
		fmt.Println("✅ Method 1 - Compact format saved to file successfully")
	}

	// ✅ Correct Method 2: Save formatted data object to file
	err = json.SaveToFile("dev_test/user_data_pretty.json", userData, true)
	if err != nil {
		log.Printf("SaveToFile failed: %v", err)
	} else {
		fmt.Println("✅ Method 2 - Formatted save to file successfully")
	}

	fmt.Println("\n3️⃣ Correct Methods for Handling JSON Strings")
	fmt.Println("==============================")

	// Assume we have a JSON string (e.g., result from Set operation)
	jsonString, _ := json.Set(`{"name":"Bob","age":25}`, "age", 26)
	fmt.Printf("JSON string: %s\n", jsonString)

	// ❌ Wrong method: Pass JSON string directly to SaveToWriter
	// This will cause the string to be double-encoded
	fmt.Println("\n❌ Wrong method demonstration:")
	var wrongBuffer bytes.Buffer
	err = processor.SaveToWriter(&wrongBuffer, jsonString, false)
	if err != nil {
		log.Printf("SaveToWriter failed: %v", err)
	} else {
		wrongResult := wrongBuffer.String()
		fmt.Printf("Wrong result (double-encoded): %s\n", wrongResult)
	}

	// ✅ Correct Method 1: Write JSON string directly to Buffer
	fmt.Println("\n✅ Correct Method 1 - Write string directly:")
	var correctBuffer1 bytes.Buffer
	_, err = correctBuffer1.WriteString(jsonString)
	if err != nil {
		log.Printf("Write failed: %v", err)
	} else {
		fmt.Printf("Correct result: %s\n", correctBuffer1.String())
	}

	// ✅ Correct Method 2: Parse JSON string to object, then save
	fmt.Println("\n✅ Correct Method 2 - Parse then save:")
	var parsedData any
	err = processor.Parse(jsonString, &parsedData)
	if err != nil {
		log.Printf("Parse failed: %v", err)
	} else {
		var correctBuffer2 bytes.Buffer
		err = processor.SaveToWriter(&correctBuffer2, parsedData, false)
		if err != nil {
			log.Printf("SaveToWriter failed: %v", err)
		} else {
			fmt.Printf("Correct result: %s\n", correctBuffer2.String())
		}
	}

	// ✅ Correct Method 3: Use os.WriteFile directly to save JSON string
	fmt.Println("\n✅ Correct Method 3 - Save string directly to file:")
	err = os.WriteFile("dev_test/direct_save.json", []byte(jsonString), 0644)
	if err != nil {
		log.Printf("Write file failed: %v", err)
	} else {
		fmt.Println("✅ JSON string saved directly to file successfully")
	}

	fmt.Println("\n4️⃣ Verify File Contents")
	fmt.Println("================")

	// Verify that saved files are valid JSON
	files := []string{
		"dev_test/user_data_compact.json",
		"dev_test/user_data_pretty.json",
		"dev_test/direct_save.json",
	}

	for _, filename := range files {
		data, err := os.ReadFile(filename)
		if err != nil {
			fmt.Printf("❌ Failed to read %s: %v\n", filename, err)
			continue
		}

		var testData any
		err = processor.Parse(string(data), &testData)
		if err != nil {
			fmt.Printf("❌ %s is not valid JSON: %v\n", filename, err)
		} else {
			fmt.Printf("✅ %s is valid JSON, size: %d bytes\n", filename, len(data))
		}
	}

	fmt.Println("\n📋 Summary:")
	fmt.Println("- ✅ Use SaveToWriter/SaveToFile for saving data objects")
	fmt.Println("- ✅ For JSON strings, write directly or use os.WriteFile")
	fmt.Println("- ✅ Avoid double-encoding JSON strings")
	fmt.Println("- ✅ Choose compact or formatted output as needed")
}
