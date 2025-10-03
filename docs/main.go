package main

import (
	"fmt"

	"github.com/cybergodev/json"
)

func main() {
	jsonData := `{"user": {"name": "Alice"}}`

	// Missing field
	email, err := json.GetString(jsonData, "user.email")
	// err = ErrPathNotFound
	fmt.Println(email, err)

	// Null field
	jsonData2 := `{"user": {"name": "Alice", "email": null}}`
	email2, err2 := json.GetString(jsonData2, "user.email")
	// email2 = "", err2 = nil (null converts to empty string)
	fmt.Println(email2, err2)
}
