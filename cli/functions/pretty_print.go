package functions

import (
	"encoding/json"
	"fmt"
	"log"
)

// PrettyPrint - print JSON with indentation
func PrettyPrint(data any) {
	body, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(string(body))
}
