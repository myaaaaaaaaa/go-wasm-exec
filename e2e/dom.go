//go:build ignore

package main

import (
	"fmt"
	"syscall/js"
)

func main() {
	document := js.Global().Get("document")
	if document.IsUndefined() || document.IsNull() {
		fmt.Println("No document found (not running in browser?)")
		return
	}
	body := document.Get("body")
	el := document.Call("createElement", "div")
	el.Set("innerHTML", "Hello from DOM!")
	body.Call("appendChild", el)
	fmt.Println("DOM manipulated successfully")
}
