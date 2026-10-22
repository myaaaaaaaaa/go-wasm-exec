package main

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"text/template"
)

const jsTemplate = `{{.WasmExec}}

// Polyfills and environment setup
globalThis.require = require;
globalThis.fs = require("fs");
globalThis.path = require("path");
globalThis.TextEncoder = require("util").TextEncoder;
globalThis.TextDecoder = require("util").TextDecoder;
globalThis.performance ??= require("performance");
globalThis.crypto ??= require("crypto");

// Wasm binary base64
const wasmBase64 = "{{.WasmBase64}}";
const wasmBinary = Buffer.from(wasmBase64, 'base64');

const go = new Go();
// Handle argv correctly for piped script (node - arg1 arg2)
// node puts '-' as process.argv[1] when reading from stdin
let scriptArgs = process.argv.slice(1);
if (scriptArgs.length > 0 && scriptArgs[0] === '-') {
    scriptArgs = scriptArgs.slice(1);
} else {
    // If not '-', fallback to typical slice(2) just in case
    if (process.argv.length > 2 && scriptArgs[0] === process.argv[1]) {
       // if scriptArgs[0] is the script path, shift it out
       scriptArgs = scriptArgs.slice(1);
    } else {
       scriptArgs = [];
    }
}
// process.argv[0] is typically the node executable itself
// go.argv expects the first argument to be the executable
go.argv = [process.argv[0], ...scriptArgs];

go.env = Object.assign({ TMPDIR: require("os").tmpdir() }, process.env);
go.exit = process.exit;

WebAssembly.instantiate(wasmBinary, go.importObject).then((result) => {
	process.on("exit", (code) => { // Node.js exits if no event handler is pending
		if (code === 0 && !go.exited) {
			// deadlock, make Go print error and stack traces
			go._pendingEvent = { id: 0 };
			go._resume();
		}
	});
	return go.run(result.instance);
}).catch((err) => {
	console.error(err);
	process.exit(1);
});
`

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: %s <binary.wasm>\n", os.Args[0])
		os.Exit(1)
	}

	wasmFile := os.Args[1]
	wasmBytes, err := os.ReadFile(wasmFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading %s: %v\n", wasmFile, err)
		os.Exit(1)
	}

	wasmBase64 := base64.StdEncoding.EncodeToString(wasmBytes)

	// find wasm_exec.js
	goroot, err := exec.Command("go", "env", "GOROOT").Output()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting GOROOT: %v\n", err)
		os.Exit(1)
	}
	
	wasmExecPath := filepath.Join(string(bytes.TrimSpace(goroot)), "lib", "wasm", "wasm_exec.js")
	wasmExecBytes, err := os.ReadFile(wasmExecPath)
	if err != nil {
		// Try older path if it's there
		wasmExecPath = filepath.Join(string(bytes.TrimSpace(goroot)), "misc", "wasm", "wasm_exec.js")
		wasmExecBytes, err = os.ReadFile(wasmExecPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading wasm_exec.js: %v\n", err)
			os.Exit(1)
		}
	}

	t := template.Must(template.New("js").Parse(jsTemplate))
	err = t.Execute(os.Stdout, struct {
		WasmExec   string
		WasmBase64 string
	}{
		WasmExec:   string(wasmExecBytes),
		WasmBase64: wasmBase64,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error generating JS: %v\n", err)
		os.Exit(1)
	}
}
