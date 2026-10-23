//go:build ignore

package main

import (
	"fmt"
	"syscall/js"
)

func main() {
	document := js.Global().Get("document")
	body := document.Get("body")
	el := document.Call("createElement", "div")
	el.Set("innerHTML", "Hello from DOM!")
	body.Call("appendChild", el)
	fmt.Println(el.Get("outerHTML").String())
}
