package main

import (
	"bufio"
	"bytes"
	"context"
	_ "embed"
	"encoding/base64"
	"encoding/json"
	"fmt"
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

//go:embed default.html
var defaultHTMLTemplate string

const jsTemplate = `
const outputBuffer = [];
const origLog = console.log;
console.log = (...args) => {
	origLog.apply(console, args);
	outputBuffer.push(args.join(" "));
};

{{.WasmExec}}

const wasmBase64 = "{{.WasmBase64}}";
const bytes = Uint8Array.fromBase64(wasmBase64);

const exitSentinelPrefix = "{{.ExitSentinelPrefix}}";

let exitCode = 0;

const go = new Go();
go.argv = {{.ArgsJSON}};
go.exit = (code) => { exitCode = code };

try {
	const result = await WebAssembly.instantiate(bytes.buffer, go.importObject);
	await go.run(result.instance);
} catch (err) {
	console.error(err);
	exitCode = 1;
}

console.log(exitSentinelPrefix + exitCode);

const pre = document.createElement("pre");
pre.textContent = outputBuffer.join("\n");
document.body.appendChild(pre);

`

// BundleWasmToHTML takes a path to a WASM file and any runtime arguments,
// generating a self-contained HTML string that runs the WASM bundle.
// If an error occurs, it logs to stderr and exits via exitf.
func BundleWasmToHTML(wasmFilePath string, htmlTemplate string, wasmArgs []string) string {
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

	t := template.Must(template.New("js").Parse(jsTemplate))
	var buf bytes.Buffer
	err = t.Execute(&buf, map[string]string{
		"WasmExec":           string(wasmExecBytes),
		"WasmBase64":         wasmBase64,
		"ArgsJSON":           string(argsJSON),
		"ExitSentinelPrefix": exitSentinelPrefix,
	})
	if err != nil {
		exitf("Error generating JavaScript: %v\n", err)
	}

	goscriptRegex := regexp.MustCompile(`<goscript.*?>`)
	scriptTag := "<script type=\"module\">" + buf.String() + "</script>"
	return goscriptRegex.ReplaceAllLiteralString(htmlTemplate, scriptTag)
}

// sendCDP sends a command over WebSocket and blocks until the response with matching id is received.
func sendCDP(ctx context.Context, c *websocket.Conn, id int, method string, params any, result any) error {
	req := map[string]any{
		"id":     id,
		"method": method,
		"params": params,
	}
	if err := wsjson.Write(ctx, c, req); err != nil {
		return err
	}

	for {
		var resp struct {
			ID     int             `json:"id"`
			Error  any             `json:"error"`
			Result json.RawMessage `json:"result"`
		}
		if err := wsjson.Read(ctx, c, &resp); err != nil {
			return err
		}
		if resp.ID != id {
			continue
		}
		if resp.Error != nil {
			return fmt.Errorf("CDP error: %v", resp.Error)
		}

		return json.Unmarshal(resp.Result, result)
	}
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

	chromeBin := FindChrome()
	if chromeBin == "" {
		exitf("Error: Chrome or Chromium executable not found\n\n%s", ChromeInstallInstructions)
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
	// Satisfy the linter
	_ = scanner.Err()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	defer cancel()

	c, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		exitf("Error dialing browser ws: %v\n", err)
	}
	defer c.Close(websocket.StatusNormalClosure, "")

	var createTarget struct {
		TargetID string `json:"targetId"`
	}
	if err := sendCDP(ctx, c, 1, "Target.createTarget", map[string]any{"url": "about:blank"}, &createTarget); err != nil {
		exitf("Error creating page target: %v\n", err)
	}

	var attachTarget struct {
		SessionID string `json:"sessionId"`
	}
	if err := sendCDP(ctx, c, 2, "Target.attachToTarget", map[string]any{
		"targetId": createTarget.TargetID,
		"flatten":  true,
	}, &attachTarget); err != nil {
		exitf("Error attaching to page target: %v\n", err)
	}

	sessionID := attachTarget.SessionID

	wsjson.Write(ctx, c, map[string]any{
		"id":        3,
		"sessionId": sessionID,
		"method":    "Runtime.enable",
	})
	wsjson.Write(ctx, c, map[string]any{
		"id":        4,
		"sessionId": sessionID,
		"method":    "Page.enable",
	})

	fileURL := "file://" + filepath.ToSlash(tmpFile.Name())
	wsjson.Write(ctx, c, map[string]any{
		"id":        5,
		"sessionId": sessionID,
		"method":    "Page.navigate",
		"params": map[string]any{
			"url": fileURL,
		},
	})

	for {
		var msg struct {
			Method    string `json:"method"`
			SessionID string `json:"sessionId"`
			Params    struct {
				Args []struct {
					Type  string `json:"type"`
					Value any    `json:"value"`
				} `json:"args"`
			} `json:"params"`
		}
		err = wsjson.Read(ctx, c, &msg)
		if err != nil {
			exitf("Error reading from WebSocket: %v\n", err)
		}
		if msg.SessionID != sessionID || msg.Method != "Runtime.consoleAPICalled" {
			continue
		}

		var out []string
		for _, arg := range msg.Params.Args {
			if arg.Value != nil {
				out = append(out, fmt.Sprintf("%v", arg.Value))
			} else if arg.Type == "undefined" {
				out = append(out, "undefined")
			}
		}

		line := strings.Join(out, " ")
		if after, ok := strings.CutPrefix(line, exitSentinelPrefix); ok {
			exitCode, err := strconv.Atoi(strings.TrimSpace(after))
			if err != nil {
				exitf("Error: Could not parse exit code from WASM: %v\n", line)
			}
			return exitCode
		}
		fmt.Println(line)
	}
}

// executeWasm is a helper to combine the bundling and execution phases.
func executeWasm(wasmFile string, args []string) {
	html := BundleWasmToHTML(wasmFile, defaultHTMLTemplate, args)
	code := RunHTMLInChrome(html)
	os.Exit(code)
}

func isWasm(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()

	var buf [4]byte
	if _, err := f.Read(buf[:]); err != nil {
		return false
	}

	return string(buf[:]) == "\x00asm"
}

func main() {
	if len(os.Args) < 2 {
		bin := os.Args[0]
		fmt.Fprintf(os.Stderr, "Usage:\n")
		fmt.Fprintf(os.Stderr, "  %s install                        - Prints commands to install Chromium\n", bin)
		fmt.Fprintf(os.Stderr, "  %s bundle <binary.wasm> [template.html] [args...] - Prints the bundled HTML to stdout\n", bin)
		fmt.Fprintf(os.Stderr, "  %s run <binary.wasm> [args...]    - Runs the WASM file in headless Chrome\n", bin)
		fmt.Fprintf(os.Stderr, "  %s <binary.wasm> [args...]        - Runs the WASM file (go_js_wasm_exec compatible)\n", bin)
		os.Exit(1)
	}

	command := os.Args[1]

	switch command {
	case "bundle":
		// Subcommand: Print the bundled HTML to stdout instead of running it.
		bundleArgs := os.Args[2:]
		var wasmFile string
		var htmlFile string
		htmlTemplate := defaultHTMLTemplate
		var args []string

		if len(bundleArgs) == 0 {
			exitf("Usage: %s bundle <binary.wasm> [template.html] [args...]\n", os.Args[0])
		} else if len(bundleArgs) == 1 {
			wasmFile = bundleArgs[0]
			if !isWasm(wasmFile) {
				exitf("Error: %s is not a WASM file (missing magic header)\n", wasmFile)
			}
		} else {
			arg1 := bundleArgs[0]
			arg2 := bundleArgs[1]

			if isWasm(arg1) {
				wasmFile = arg1
				htmlFile = arg2
			} else if isWasm(arg2) {
				wasmFile = arg2
				htmlFile = arg1
			} else {
				exitf("Error: neither %s nor %s are a WASM file (missing magic header)\n", arg1, arg2)
			}

			b, err := os.ReadFile(htmlFile)
			if err != nil {
				exitf("Error reading HTML template: %v\n", err)
			}
			htmlTemplate = string(b)

			args = bundleArgs[2:]
		}

		html := BundleWasmToHTML(wasmFile, htmlTemplate, args)

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

	case "install":
		// Subcommand: Print one-liners to install Chromium.
		message := ChromeInstallInstructions
		if chromePath := FindChrome(); chromePath != "" {
			message = "Chromium/Chrome is already installed at: " + chromePath + "\nYou are good to go!\n"
		}
		fmt.Print(message)

	default:
		// go_js_wasm_exec compatibility mode.
		// `go run` or `go test` will call this binary directly with the
		// `.wasm` file as the first argument (no subcommand).
		wasmFile := os.Args[1]
		args := os.Args[2:]

		executeWasm(wasmFile, args)
	}
}
