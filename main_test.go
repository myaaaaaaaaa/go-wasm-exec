package main

import (
	"encoding/xml"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

var scriptTagRegex = regexp.MustCompile(`(?s)<script.*?</script.*?>`)

func checkBundleWasmToHTML(t *testing.T, wasmFilePath string, wasmArgs []string) string {
	t.Helper()
	html := BundleWasmToHTML(wasmFilePath, wasmArgs)

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
		if html == "" {
			t.Fatal("BundleWasmToHTML() returned empty HTML string")
		}

		if !strings.Contains(html, "test.wasm") {
			t.Fatalf("expected bundled HTML to contain filename 'test.wasm'")
		}
	})

	t.Run("with args", func(t *testing.T) {
		args := []string{"--test-flag", "value"}
		html := checkBundleWasmToHTML(t, wasmFile, args)
		if html == "" {
			t.Fatal("BundleWasmToHTML() returned empty HTML string")
		}
		for _, arg := range args {
			if !strings.Contains(html, arg) {
				t.Fatalf("expected bundled HTML to contain argument %q", arg)
			}
		}
	})
}
