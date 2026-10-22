# go-wasm-exec

`go-wasm-exec` is a headless-browser runner and bundler for Go WebAssembly binaries (`GOOS=js GOARCH=wasm`), designed to act as a drop-in execution tool with `go run -exec` and `go test -exec`. It embeds compiled WASM and the Go runtime (`wasm_exec.js`) into a self-contained HTML page, executes it inside headless Chrome/Chromium streaming console output to stdout, and propagates the program's exit code back to the caller.

## Testing

Ensure a Chrome or Chromium browser is installed (or pointed to via the `CHROME_BIN` environment variable), then run:

* Run all tests: `go test -v ./...`
* Run tests without caching: `go test -v -count=1 ./...`

Note that the `./...` pattern is required to run the end-to-end tests in `e2e`, which are kept separate and omitted by default during root package test runs due to their overhead.
