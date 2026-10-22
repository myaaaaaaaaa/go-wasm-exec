package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"text/template"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
)

// exitf prints a formatted string to stderr and exits with status 1.
func exitf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format, args...)
	os.Exit(1)
}

const exitSentinelPrefix = "__GO_WASM_EXIT__:"

const htmlTemplate = `<!DOCTYPE html>
<html>
<head>
	<meta charset="utf-8">
	<title>Go WASM</title>
</head>
<body>
	<script>
		{{.WasmExec}}
	</script>
	<script>
		const wasmBase64 = "{{.WasmBase64}}";
		const binaryString = atob(wasmBase64);
		const bytes = new Uint8Array(binaryString.length);
		for (let i = 0; i < binaryString.length; i++) {
			bytes[i] = binaryString.charCodeAt(i);
		}

		const go = new Go();
		go.argv = {{.ArgsJSON}};

		const exitSentinelPrefix = "{{.ExitSentinelPrefix}}";
		let exited = false;
		const handleExit = (code) => {
			if (exited) return;
			exited = true;
			console.log(exitSentinelPrefix + (code ?? 0));
		};

		// Map console.log to standard output and exit when process ends
		go.exit = (code) => {
			if (code !== 0) {
				console.error("exit code:", code);
			}
			handleExit(code);
		};

		WebAssembly.instantiate(bytes.buffer, go.importObject).then(async (result) => {
			await go.run(result.instance);
			handleExit(0);
		}).catch((err) => {
			console.error(err);
			handleExit(1);
		});
	</script>
</body>
</html>`

// BundleWasmToHTML takes a path to a WASM file and any runtime arguments,
// generating a self-contained HTML string that runs the WASM bundle.
// If an error occurs, it logs to stderr and exits via exitf.
func BundleWasmToHTML(wasmFilePath string, wasmArgs []string) string {
	wasmBytes, err := os.ReadFile(wasmFilePath)
	if err != nil {
		exitf("Error reading %s: %v\n", wasmFilePath, err)
	}

	wasmBase64 := base64.StdEncoding.EncodeToString(wasmBytes)

	// find wasm_exec.js
	goroot, err := exec.Command("go", "env", "GOROOT").Output()
	if err != nil {
		exitf("Error getting GOROOT: %v\n", err)
	}

	gorootDir := string(bytes.TrimSpace(goroot))
	candidates := []string{
		filepath.Join(gorootDir, "lib", "wasm", "wasm_exec.js"),
		filepath.Join(gorootDir, "misc", "wasm", "wasm_exec.js"),
	}

	var wasmExecBytes []byte
	for _, path := range candidates {
		if b, err := os.ReadFile(path); err == nil {
			wasmExecBytes = b
			break
		}
	}
	if wasmExecBytes == nil {
		exitf("Error reading wasm_exec.js\n")
	}

	// We pass ["js", ...] as go_js_wasm_exec traditionally does
	// or ["name", args...]
	fullArgs := append([]string{filepath.Base(wasmFilePath)}, wasmArgs...)
	argsJSON, err := json.Marshal(fullArgs)
	if err != nil {
		exitf("Error marshaling args: %v\n", err)
	}

	t := template.Must(template.New("html").Parse(htmlTemplate))
	var buf bytes.Buffer
	err = t.Execute(&buf, struct {
		WasmExec           string
		WasmBase64         string
		ArgsJSON           string
		ExitSentinelPrefix string
	}{
		WasmExec:           string(wasmExecBytes),
		WasmBase64:         wasmBase64,
		ArgsJSON:           string(argsJSON),
		ExitSentinelPrefix: exitSentinelPrefix,
	})
	if err != nil {
		exitf("Error generating HTML: %v\n", err)
	}

	return buf.String()
}

// RunHTMLInChrome takes a self-contained HTML string, executes it in
// headless Chrome, and pipes the console/websocket output to stdout.
// If an error occurs, it logs to stderr and exits via exitf.
func RunHTMLInChrome(htmlContent string) int {
	tmpFile, err := os.CreateTemp("", "htmleval-*.html")
	if err != nil {
		exitf("Error creating temp file: %v\n", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(htmlContent); err != nil {
		exitf("Error writing temp file: %v\n", err)
	}
	tmpFile.Close()

	userDataDir, err := os.MkdirTemp(os.Getenv("XDG_RUNTIME_DIR"), "htmleval-user-data-*")
	if err != nil {
		exitf("Error creating user data dir: %v\n", err)
	}
	defer os.RemoveAll(userDataDir)

	args := []string{
		"--no-first-run",
		"--no-default-browser-check",
		"--headless",
		"--disable-background-networking",
		"--enable-features=NetworkService,NetworkServiceInProcess",
		"--disable-background-timer-throttling",
		"--disable-backgrounding-occluded-windows",
		"--disable-breakpad",
		"--disable-client-side-phishing-detection",
		"--disable-default-apps",
		"--disable-dev-shm-usage",
		"--disable-extensions",
		"--disable-features=Translate,BlinkGenPropertyTrees",
		"--disable-hang-monitor",
		"--disable-ipc-flooding-protection",
		"--disable-popup-blocking",
		"--disable-prompt-on-repost",
		"--disable-renderer-backgrounding",
		"--disable-sync",
		"--force-color-profile=srgb",
		"--metrics-recording-only",
		"--safebrowsing-disable-auto-update",
		"--enable-automation",
		"--password-store=basic",
		"--use-mock-keychain",
		"--remote-debugging-port=0",
		"--user-data-dir=" + userDataDir,
		"about:blank",
	}

	candidates := []string{
		"chromium",
		"google-chrome",
		"chromium-browser",
	}
	chromeBin := candidates[0]
	for _, bin := range candidates {
		if _, err := exec.LookPath(bin); err == nil {
			chromeBin = bin
			break
		}
	}

	cmd := exec.Command(chromeBin, args...)
	stderr, err := cmd.StderrPipe()
	if err != nil {
		exitf("Error piping stderr: %v\n", err)
	}

	if err := cmd.Start(); err != nil {
		exitf("Error starting chrome: %v\n", err)
	}
	defer func() {
		cmd.Process.Kill()
		cmd.Wait()
	}()

	scanner := bufio.NewScanner(stderr)
	var wsURL string
	re := regexp.MustCompile(`ws://[^\s]+`)
	for scanner.Scan() {
		line := scanner.Text()
		if match := re.FindString(line); match != "" {
			wsURL = match
			break
		}
	}

	if wsURL == "" {
		exitf("Error: Could not find WebSocket URL in chrome output\n")
	}

	portRe := regexp.MustCompile(`127\.0\.0\.1:(\d+)`)
	portMatch := portRe.FindStringSubmatch(wsURL)
	if len(portMatch) < 2 {
		exitf("Error: Could not extract port from WebSocket URL\n")
	}

	port := portMatch[1]

	var targets []map[string]any
	var pageWs string
	for range 20 {
		resp, err := http.Get("http://127.0.0.1:" + port + "/json/list")
		if err == nil {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			json.Unmarshal(body, &targets)
			for _, t := range targets {
				if t["type"] == "page" {
					if ws, ok := t["webSocketDebuggerUrl"].(string); ok && ws != "" {
						pageWs = ws
						break
					}
				}
			}
			if pageWs != "" {
				break
			}
		}
		time.Sleep(50 * time.Millisecond)
	}

	if pageWs == "" {
		exitf("Error: Could not find page target WebSocket URL\n")
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	defer cancel()

	c, _, err := websocket.Dial(ctx, pageWs, nil)
	if err != nil {
		exitf("Error dialing page ws: %v\n", err)
	}
	defer c.Close(websocket.StatusNormalClosure, "")

	wsjson.Write(ctx, c, map[string]any{
		"id":     1,
		"method": "Runtime.enable",
	})
	wsjson.Write(ctx, c, map[string]any{
		"id":     2,
		"method": "Page.enable",
	})

	fileURL := "file://" + filepath.ToSlash(tmpFile.Name())
	wsjson.Write(ctx, c, map[string]any{
		"id":     3,
		"method": "Page.navigate",
		"params": map[string]any{
			"url": fileURL,
		},
	})

	exitCode := 1

	for {
		var v map[string]any
		err = wsjson.Read(ctx, c, &v)
		if err != nil {
			break
		}

		if method, ok := v["method"].(string); ok {
			if method == "Runtime.consoleAPICalled" {
				if params, ok := v["params"].(map[string]any); ok {
					if args, ok := params["args"].([]any); ok {
						var out []string
						for _, arg := range args {
							if a, ok := arg.(map[string]any); ok {
								if val, ok := a["value"]; ok {
									out = append(out, fmt.Sprintf("%v", val))
								} else if t, ok := a["type"].(string); ok && t == "undefined" {
									out = append(out, "undefined")
								} else {
									out = append(out, fmt.Sprintf("%v", a))
								}
							}
						}
						line := strings.Join(out, " ")
						if after, ok0 := strings.CutPrefix(line, exitSentinelPrefix); ok0 {
							codeStr := after
							if c, err := strconv.Atoi(strings.TrimSpace(codeStr)); err == nil {
								exitCode = c
							}
							return exitCode
						}
						fmt.Println(line)
					}
				}
			}
		}
	}

	return exitCode
}

// executeWasm is a helper to combine the bundling and execution phases.
func executeWasm(wasmFile string, args []string) {
	html := BundleWasmToHTML(wasmFile, args)
	code := RunHTMLInChrome(html)
	os.Exit(code)
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
