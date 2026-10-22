package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: %s \"<html>...</html>\"\n", os.Args[0])
		os.Exit(1)
	}
	htmlContent := os.Args[1]

	tmpFile, err := os.CreateTemp("", "htmleval-*.html")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating temp file: %v\n", err)
		os.Exit(1)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(htmlContent); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing temp file: %v\n", err)
		os.Exit(1)
	}
	tmpFile.Close()

	userDataDir, err := os.MkdirTemp(os.Getenv("XDG_RUNTIME_DIR"), "htmleval-user-data-*")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating user data dir: %v\n", err)
		os.Exit(1)
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

	chromeBin := "google-chrome"
	if _, err := exec.LookPath("google-chrome"); err != nil {
		if _, err := exec.LookPath("chromium"); err == nil {
			chromeBin = "chromium"
		} else if _, err := exec.LookPath("chromium-browser"); err == nil {
			chromeBin = "chromium-browser"
		}
	}

	cmd := exec.Command(chromeBin, args...)
	stderr, err := cmd.StderrPipe()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error piping stderr: %v\n", err)
		os.Exit(1)
	}

	if err := cmd.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "Error starting chrome: %v\n", err)
		os.Exit(1)
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
		fmt.Fprintln(os.Stderr, "Error: Could not find WebSocket URL in chrome output")
		os.Exit(1)
	}

	portRe := regexp.MustCompile(`127\.0\.0\.1:(\d+)`)
	portMatch := portRe.FindStringSubmatch(wsURL)
	if len(portMatch) < 2 {
		fmt.Fprintln(os.Stderr, "Error: Could not extract port from WebSocket URL")
		os.Exit(1)
	}

	port := portMatch[1]
	
	var targets []map[string]interface{}
	var pageWs string
	for i := 0; i < 20; i++ {
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
		fmt.Fprintln(os.Stderr, "Error: Could not find page target WebSocket URL")
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	defer cancel()

	c, _, err := websocket.Dial(ctx, pageWs, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error dialing page ws: %v\n", err)
		os.Exit(1)
	}
	defer c.Close(websocket.StatusNormalClosure, "")

	wsjson.Write(ctx, c, map[string]interface{}{
		"id":     1,
		"method": "Runtime.enable",
	})
	wsjson.Write(ctx, c, map[string]interface{}{
		"id":     2,
		"method": "Page.enable",
	})
	
	fileURL := "file://" + filepath.ToSlash(tmpFile.Name())
	wsjson.Write(ctx, c, map[string]interface{}{
		"id":     3,
		"method": "Page.navigate",
		"params": map[string]interface{}{
			"url": fileURL,
		},
	})
	
	loadEventFired := false
	var loadEventTime time.Time
	
	for {
		var v map[string]interface{}
		err = wsjson.Read(ctx, c, &v)
		if err != nil {
			break
		}

		if method, ok := v["method"].(string); ok {
			if method == "Runtime.consoleAPICalled" {
				if params, ok := v["params"].(map[string]interface{}); ok {
					if args, ok := params["args"].([]interface{}); ok {
						var out []string
						for _, arg := range args {
							if a, ok := arg.(map[string]interface{}); ok {
								if val, ok := a["value"]; ok {
									out = append(out, fmt.Sprintf("%v", val))
								} else if t, ok := a["type"].(string); ok && t == "undefined" {
									out = append(out, "undefined")
								} else {
									out = append(out, fmt.Sprintf("%v", a))
								}
							}
						}
						fmt.Println(strings.Join(out, " "))
					}
				}
			} else if method == "Page.loadEventFired" {
				loadEventFired = true
				loadEventTime = time.Now()
			}
		}

		if loadEventFired && time.Since(loadEventTime) > 100*time.Millisecond {
			// To be safe, wait slightly after loadEventFired for any final synchronous or microtask logs
			time.Sleep(100 * time.Millisecond)
			break
		}
	}
}
