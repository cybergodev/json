package main

import (
	"fmt"

	"github.com/cybergodev/json"
)

// Temporary test case

func main() {

	arrayData := `{
	  "numbers": [1, 2, 3, 4, 5, 6, 7, 8, 9, 10],
	  "users": [
		{"name": "Alice", "age": 25},
		{"name": "Bob", "age": 30}
	  ]
	}`

	step, _ := json.Get(arrayData, "numbers[::-2]") // [10 8 6 4 2]
	fmt.Println(step)

}
