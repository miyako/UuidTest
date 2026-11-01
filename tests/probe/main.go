// This host-test executable is never distributed with the component.
package main

import (
	"encoding/json"
	"fmt"
	"os"
)

func main() {
	fmt.Fprintln(os.Stderr, "probe diagnostic")
	if err := json.NewEncoder(os.Stdout).Encode(struct {
		Arguments []string `json:"arguments"`
	}{os.Args[1:]}); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
