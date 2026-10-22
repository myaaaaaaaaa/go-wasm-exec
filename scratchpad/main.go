package main

import (
	"fmt"
	"os"
)

// exitf prints a formatted string to stderr and exits with status 1.
func exitf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format, args...)
	os.Exit(1)
}

// BundleWasmToHTML takes a path to a WASM file and any runtime arguments,
// generating a self-contained HTML string that runs the WASM bundle.
// If an error occurs, it logs to stderr and exits via exitf.
func BundleWasmToHTML(wasmFilePath string, wasmArgs []string) string {
	// Port the logic from the `gowasmbundle`
	// IMPORTANT: `gobundle` currently generates a Node.js script. You must modify the 
	//    template to output a valid HTML document that avoids node-specific APIs.
	return "<html>...</html>"
}

// RunHTMLInChrome takes a self-contained HTML string, executes it in
// headless Chrome, and pipes the console/websocket output to stdout.
// If an error occurs, it logs to stderr and exits via exitf.
func RunHTMLInChrome(htmlContent string) {
	// Port the logic from the `htmleval`
}

// executeWasm is a helper to combine the bundling and execution phases.
func executeWasm(wasmFile string, args []string) {
	html := BundleWasmToHTML(wasmFile, args)
	RunHTMLInChrome(html)
}

func main() {
	if len(os.Args) < 2 {
		bin := os.Args[0]
		fmt.Fprintf(os.Stderr, "Usage:\n")
		fmt.Fprintf(os.Stderr, "  %s bundle <binary.wasm> [args...] - Prints the bundled HTML to stdout\n", bin)
		fmt.Fprintf(os.Stderr, "  %s run <binary.wasm> [args...]    - Runs the WASM file in headless Chrome\n", bin)
		fmt.Fprintf(os.Stderr, "  %s <binary.wasm> [args...]        - Runs the WASM file (go_js_wasm_exec compatible)\n", bin)
		os.Exit(1)
	}

	command := os.Args[1]

	switch command {
	case "bundle":
		// Subcommand: Print the bundled HTML to stdout instead of running it.
		if len(os.Args) < 3 {
			exitf("Usage: %s bundle <binary.wasm> [args...]\n", os.Args[0])
		}
		
		wasmFile := os.Args[2]
		args := os.Args[3:]
		
		html := BundleWasmToHTML(wasmFile, args)
		
		// Print directly to stdout
		fmt.Println(html)

	case "run":
		// Subcommand: Explicitly run the WASM file in Chrome.
		if len(os.Args) < 3 {
			exitf("Usage: %s run <binary.wasm> [args...]\n", os.Args[0])
		}
		
		wasmFile := os.Args[2]
		args := os.Args[3:]
		
		executeWasm(wasmFile, args)

	default:
		// go_js_wasm_exec compatibility mode.
		// `go run` or `go test` will call this binary directly with the 
		// `.wasm` file as the first argument (no subcommand).
		wasmFile := os.Args[1]
		args := os.Args[2:]
		
		executeWasm(wasmFile, args)
	}
}