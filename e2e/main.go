//go:build ignore

package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Println("Hello, WASM!")
	fmt.Println("Args:", os.Args)
}
