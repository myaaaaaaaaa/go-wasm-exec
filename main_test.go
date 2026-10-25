package main

import (
	"encoding/xml"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
)

func checkBundleWasmToHTML(t *testing.T, wasmFilePath string, wasmArgs []string) string {
	t.Helper()
	html := BundleWasmToHTML(wasmFilePath, defaultHTMLTemplate, wasmArgs)
	if html == "" {
		t.Fatal("BundleWasmToHTML() returned empty HTML string")
	}
	if strings.Contains(html, "<goscript") {
		t.Fatalf("expected <goscript /> tag to be replaced in bundled HTML")
	}

	{
		// HTML parses the inside of <script> tags differently than a generic XML parser.
		// We need to strip it out for XML validation.
		scriptTagRegex := regexp.MustCompile(`(?s)<script.*?</script.*?>`)
		strippedHTML := scriptTagRegex.ReplaceAllString(html, "")
		decoder := xml.NewDecoder(strings.NewReader(strippedHTML))
		for {
			_, err := decoder.Token()
			if err == io.EOF {
				break
			}
			if err != nil {
				t.Fatalf("BundleWasmToHTML() produced invalid XML/XHTML: %v", err)
			}
		}
	}

	return html
}

func TestBundleWasmToHTML(t *testing.T) {
	dummyContent := []byte("\x00asm\x01\x00\x00\x00")
	wasmFile := filepath.Join(t.TempDir(), "test.wasm")
	if err := os.WriteFile(wasmFile, dummyContent, 0644); err != nil {
		t.Fatalf("failed to create dummy wasm file: %v", err)
	}

	t.Run("without args", func(t *testing.T) {
		html := checkBundleWasmToHTML(t, wasmFile, nil)
		if !strings.Contains(html, "test.wasm") {
			t.Fatalf("expected bundled HTML to contain filename 'test.wasm'")
		}
	})

	t.Run("with args", func(t *testing.T) {
		args := []string{"--test-flag", "value"}
		html := checkBundleWasmToHTML(t, wasmFile, args)
		for _, arg := range args {
			if !strings.Contains(html, arg) {
				t.Fatalf("expected bundled HTML to contain argument %q", arg)
			}
		}
	})
}

func TestParseBundleArgs(t *testing.T) {
	dummyContent := []byte("\x00asm\x01\x00\x00\x00")
	tmpDir := t.TempDir()
	wasmFile := filepath.Join(tmpDir, "test.wasm")
	if err := os.WriteFile(wasmFile, dummyContent, 0644); err != nil {
		t.Fatalf("failed to create dummy wasm file: %v", err)
	}

	htmlContent := "<html><body>custom</body></html>"
	htmlFile := filepath.Join(tmpDir, "custom.html")
	if err := os.WriteFile(htmlFile, []byte(htmlContent), 0644); err != nil {
		t.Fatalf("failed to create custom html file: %v", err)
	}

	t.Run("single wasm file", func(t *testing.T) {
		wasm, tmpl, args := parseBundleArgs([]string{wasmFile})
		if wasm != wasmFile {
			t.Errorf("got wasmFile %q, want %q", wasm, wasmFile)
		}
		if tmpl != defaultHTMLTemplate {
			t.Errorf("got template %q, want default template", tmpl)
		}
		if len(args) != 0 {
			t.Errorf("got args %v, want empty", args)
		}
	})

	t.Run("wasm file and html template", func(t *testing.T) {
		wasm, tmpl, args := parseBundleArgs([]string{wasmFile, htmlFile})
		if wasm != wasmFile {
			t.Errorf("got wasmFile %q, want %q", wasm, wasmFile)
		}
		if tmpl != htmlContent {
			t.Errorf("got template %q, want %q", tmpl, htmlContent)
		}
		if len(args) != 0 {
			t.Errorf("got args %v, want empty", args)
		}
	})

	t.Run("html template and wasm file reversed", func(t *testing.T) {
		wasm, tmpl, args := parseBundleArgs([]string{htmlFile, wasmFile})
		if wasm != wasmFile {
			t.Errorf("got wasmFile %q, want %q", wasm, wasmFile)
		}
		if tmpl != htmlContent {
			t.Errorf("got template %q, want %q", tmpl, htmlContent)
		}
		if len(args) != 0 {
			t.Errorf("got args %v, want empty", args)
		}
	})

	t.Run("with args", func(t *testing.T) {
		extraArgs := []string{"--foo", "bar"}
		bundleArgs := append([]string{wasmFile, htmlFile}, extraArgs...)
		wasm, tmpl, args := parseBundleArgs(bundleArgs)
		if wasm != wasmFile {
			t.Errorf("got wasmFile %q, want %q", wasm, wasmFile)
		}
		if tmpl != htmlContent {
			t.Errorf("got template %q, want %q", tmpl, htmlContent)
		}
		if !slices.Equal(args, extraArgs) {
			t.Errorf("got args %v, want %v", args, extraArgs)
		}
	})

	t.Run("nested into BundleWasmToHTML", func(t *testing.T) {
		html := BundleWasmToHTML(parseBundleArgs([]string{wasmFile}))
		if !strings.Contains(html, "test.wasm") {
			t.Fatalf("expected bundled HTML to contain filename 'test.wasm'")
		}
	})
}

func TestInstallGoJsWasmExec(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("GOBIN", tmpDir)

	dest, err := installGoJsWasmExec()
	if err != nil {
		t.Fatalf("installGoJsWasmExec() failed: %v", err)
	}

	expectedDest := filepath.Join(tmpDir, "go_js_wasm_exec")
	if dest != expectedDest {
		t.Errorf("got %q, want %q", dest, expectedDest)
	}

	content, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("failed to read installed file: %v", err)
	}

	if string(content) != denoWasmExecJS {
		t.Errorf("installed content does not match denoWasmExecJS")
	}

	info, err := os.Stat(dest)
	if err != nil {
		t.Fatalf("failed to stat installed file: %v", err)
	}

	origPerm := info.Mode().Perm()
	if err := os.Chmod(dest, origPerm|0111); err != nil {
		t.Fatalf("failed to chmod: %v", err)
	}

	info, err = os.Stat(dest)
	if err != nil {
		t.Fatalf("failed to stat again: %v", err)
	}

	newPerm := info.Mode().Perm()
	if origPerm != newPerm {
		t.Errorf("expected %s to be executable, got mode %v", dest, origPerm)
	}
}
