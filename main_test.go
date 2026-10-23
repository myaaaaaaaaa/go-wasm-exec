package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBundleWasmToHTML(t *testing.T) {
	dummyContent := []byte("\x00asm\x01\x00\x00\x00")
	wasmFile := filepath.Join(t.TempDir(), "test.wasm")
	if err := os.WriteFile(wasmFile, dummyContent, 0644); err != nil {
		t.Fatalf("failed to create dummy wasm file: %v", err)
	}

	t.Run("without args", func(t *testing.T) {
		html := BundleWasmToHTML(wasmFile, nil)
		if html == "" {
			t.Fatal("BundleWasmToHTML() returned empty HTML string")
		}

		if !strings.Contains(html, "test.wasm") {
			t.Fatalf("expected bundled HTML to contain filename 'test.wasm'")
		}
	})

	t.Run("with args", func(t *testing.T) {
		args := []string{"--test-flag", "value"}
		html := BundleWasmToHTML(wasmFile, args)
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
