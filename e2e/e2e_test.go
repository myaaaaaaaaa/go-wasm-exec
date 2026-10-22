package e2e

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestE2E(t *testing.T) {
	tmpDir := t.TempDir()
	buildCmd := exec.Command("go", "build", "-o", filepath.Join(tmpDir, "go-wasm-exec"), "..")
	buildCmd.Stdout = os.Stdout
	buildCmd.Stderr = os.Stderr
	if err := buildCmd.Run(); err != nil {
		t.Fatalf("Failed to build go-wasm-exec: %v", err)
	}

	entries, err := os.ReadDir(tmpDir)
	if err != nil {
		t.Fatalf("Failed to read temp dir: %v", err)
	}
	var execBin string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasPrefix(entry.Name(), "go-wasm-exec") {
			execBin = filepath.Join(tmpDir, entry.Name())
			break
		}
	}
	if execBin == "" {
		t.Fatalf("Failed to find built go-wasm-exec in %s", tmpDir)
	}

	t.Run("Success", func(t *testing.T) {
		runCmd := exec.Command("go", "run", "-exec="+execBin, "main.go", "flag1", "flag2")
		runCmd.Env = append(os.Environ(), "GOOS=js", "GOARCH=wasm")
		out, err := runCmd.CombinedOutput()
		if err != nil {
			t.Fatalf("Failed to run main.go with go-wasm-exec: %v\nOutput:\n%s", err, out)
		}

		output := string(out)
		if !strings.Contains(output, "Hello, WASM!") {
			t.Errorf("Expected output to contain 'Hello, WASM!', got:\n%s", output)
		}
		if !strings.Contains(output, "flag1 flag2") {
			t.Errorf("Expected output to contain 'flag1 flag2', got:\n%s", output)
		}
	})

	t.Run("ExitCode", func(t *testing.T) {
		wasmFile := filepath.Join(tmpDir, "main.wasm")
		wasmBuild := exec.Command("go", "build", "-o", wasmFile, "main.go")
		wasmBuild.Env = append(os.Environ(), "GOOS=js", "GOARCH=wasm")
		if err := wasmBuild.Run(); err != nil {
			t.Fatalf("Failed to build test wasm: %v", err)
		}

		runCmd := exec.Command(execBin, wasmFile, "fail")
		out, err := runCmd.CombinedOutput()
		if err == nil {
			t.Fatalf("Expected command to fail, but succeeded with output:\n%s", out)
		}

		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) {
			t.Fatalf("Expected *exec.ExitError, got %T: %v", err, err)
		}
		if exitErr.ExitCode() != 42 {
			t.Errorf("Expected exit code 42, got: %d", exitErr.ExitCode())
		}
		if !strings.Contains(string(out), "Exiting with failure") {
			t.Errorf("Expected output to contain 'Exiting with failure', got:\n%s", out)
		}
	})
}
