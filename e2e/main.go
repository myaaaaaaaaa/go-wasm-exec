//go:build ignore

package main

import (
	"fmt"
	"os"
	"strconv"
)

func main() {
	fmt.Println("Hello, WASM!")

	sum := 0
	for _, arg := range os.Args[1:] {
		if n, err := strconv.Atoi(arg); err == nil {
			sum += n
		}
	}
	os.Exit(sum)
}
