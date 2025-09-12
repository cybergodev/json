package main

import (
	"fmt"
	"log"
	"time"

	"github.com/cybergodev/json"
)

func main() {
	fmt.Println("🚀 Optional Configuration Parameters Example")
	fmt.Println("==========================================")

	testJSON := `{
		"user": {
			"name": "Alice",
			"age": 30,
			"active": true
		},
		"settings": {
			"theme": "dark",
			"notifications": true
		}
	}`

	// 1. No parameter call - uses default configuration
	fmt.Println("\n1️⃣ No parameter call (default configuration):")
	processor1 := json.New() // No parameters passed
	defer processor1.Close()

	name, err := processor1.Get(testJSON, "user.name")
	if err != nil {
		log.Printf("❌ Get failed: %v", err)
	} else {
		fmt.Printf("✅ User name: %v\n", name)
		// Result: Alice
	}

	// 2. Pass nil - equivalent to default configuration
	fmt.Println("\n2️⃣ Pass nil configuration:")
	processor2 := json.New(nil) // Explicitly pass nil
	defer processor2.Close()

	age, err := processor2.Get(testJSON, "user.age")
	if err != nil {
		log.Printf("❌ Get failed: %v", err)
	} else {
		fmt.Printf("✅ User age: %v\n", age)
		// Result: 30
	}

	// 3. Pass custom configuration
	fmt.Println("\n3️⃣ Pass custom configuration:")

	customConfig := json.DefaultConfig()
	customConfig.EnableCache = true
	customConfig.MaxCacheSize = 2000
	customConfig.CacheTTL = 15 * time.Minute
	customConfig.MaxConcurrency = 20
	customConfig.StrictMode = false

	processor3 := json.New(customConfig)
	defer processor3.Close()

	active, err := processor3.Get(testJSON, "user.active")
	if err != nil {
		log.Printf("❌ Get failed: %v", err)
	} else {
		fmt.Printf("✅ User active status: %v\n", active)
		// Result: true
	}

	// 4. Use predefined configurations
	fmt.Println("\n4️⃣ Use predefined configurations:")

	// High security configuration
	secureProcessor := json.New(json.HighSecurityConfig())
	defer secureProcessor.Close()

	theme, err := secureProcessor.Get(testJSON, "settings.theme")
	if err != nil {
		log.Printf("❌ Get failed: %v", err)
	} else {
		fmt.Printf("✅ Theme setting: %v\n", theme)
		// Result: dark
	}

	// Large data configuration
	largeDataProcessor := json.New(json.LargeDataConfig())
	defer largeDataProcessor.Close()

	notifications, err := largeDataProcessor.Get(testJSON, "settings.notifications")
	if err != nil {
		log.Printf("❌ Get failed: %v", err)
	} else {
		fmt.Printf("✅ Notification setting: %v\n", notifications)
		// Result: true
	}

	// 5. Demonstrate backward compatibility
	fmt.Println("\n5️⃣ Backward compatibility:")
	fmt.Println("   ✅ Existing code requires no changes")
	fmt.Println("   ✅ json.New() still works")
	fmt.Println("   ✅ json.New(config) still works")
	fmt.Println("   ✅ New json.New() no-parameter call added")

	fmt.Println("\n🎉 Optional configuration parameters demo completed!")
}
