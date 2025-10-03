package json

import (
	"errors"
	"fmt"
	"testing"
)

// TestGetStringPathNotFound tests the behavior when getting a non-existent path
func TestGetStringPathNotFound(t *testing.T) {
	jsonData := `{"user": {"name": "Alice", "age": 25}}`

	// Test 1: Get non-existent property
	t.Run("NonExistentProperty", func(t *testing.T) {
		value, err := GetString(jsonData, "user.email")
		
		fmt.Printf("Value: %q\n", value)
		fmt.Printf("Error: %v\n", err)
		fmt.Printf("Error is nil: %v\n", err == nil)
		
		if err != nil {
			fmt.Printf("Error type: %T\n", err)
			if errors.Is(err, ErrPathNotFound) {
				fmt.Println("Error is ErrPathNotFound")
			} else {
				fmt.Println("Error is NOT ErrPathNotFound")
			}
		}
		
		// According to documentation, err should be ErrPathNotFound
		if !errors.Is(err, ErrPathNotFound) {
			t.Errorf("Expected ErrPathNotFound, got: %v", err)
		}
	})

	// Test 2: Get non-existent nested property
	t.Run("NonExistentNestedProperty", func(t *testing.T) {
		value, err := GetString(jsonData, "user.profile.city")
		
		fmt.Printf("Value: %q\n", value)
		fmt.Printf("Error: %v\n", err)
		
		if !errors.Is(err, ErrPathNotFound) {
			t.Errorf("Expected ErrPathNotFound, got: %v", err)
		}
	})

	// Test 3: Get with raw Get function
	t.Run("RawGetNonExistent", func(t *testing.T) {
		value, err := Get(jsonData, "user.email")

		fmt.Printf("Value: %v\n", value)
		fmt.Printf("Error: %v\n", err)

		// Check what Get returns
		if err != nil {
			fmt.Printf("Get returned error: %v\n", err)
			if errors.Is(err, ErrPathNotFound) {
				fmt.Println("Get error is ErrPathNotFound")
			}
		} else if value == nil {
			fmt.Println("Get returned nil value with no error")
		}
	})

	// Test 4: Test with modular processor
	t.Run("ModularProcessorTest", func(t *testing.T) {
		processor := New()
		defer processor.Close()

		value, err := processor.Get(jsonData, "user.email")
		fmt.Printf("Modular Get - Value: %v, Error: %v\n", value, err)

		if err != nil && errors.Is(err, ErrPathNotFound) {
			fmt.Println("Modular processor returns ErrPathNotFound")
		}
	})
}

// TestGetIntPathNotFound tests GetInt with non-existent path
func TestGetIntPathNotFound(t *testing.T) {
	jsonData := `{"user": {"name": "Alice", "age": 25}}`

	value, err := GetInt(jsonData, "user.salary")
	
	fmt.Printf("Value: %d\n", value)
	fmt.Printf("Error: %v\n", err)
	
	if !errors.Is(err, ErrPathNotFound) {
		t.Errorf("Expected ErrPathNotFound, got: %v", err)
	}
}

