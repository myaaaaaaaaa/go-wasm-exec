//go:build ignore

package main

import (
	"fmt"
	"os"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "fail" {
		fmt.Println("Exiting with failure")
		os.Exit(42)
	}
	fmt.Println("Hello, WASM!")
	fmt.Println("Args:", os.Args)
}
