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
	// Force Windows .exe heresy on all platforms
	execBin := filepath.Join(tmpDir, "go-wasm-exec.exe")

	buildCmd := exec.Command("go", "build", "-o", execBin, "..")
	buildCmd.Stdout = os.Stdout
	buildCmd.Stderr = os.Stderr
	if err := buildCmd.Run(); err != nil {
		t.Fatalf("Failed to build go-wasm-exec: %v", err)
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

	t.Run("DOM", func(t *testing.T) {
		runCmd := exec.Command("go", "run", "-exec="+execBin, "dom.go")
		runCmd.Env = append(os.Environ(), "GOOS=js", "GOARCH=wasm")
		out, err := runCmd.CombinedOutput()
		if err != nil {
			t.Fatalf("Failed to run dom.go with go-wasm-exec: %v\nOutput:\n%s", err, out)
		}

		output := string(out)
		if !strings.Contains(output, "<div>Hello from DOM!</div>") {
			t.Errorf("Expected output to contain '<div>Hello from DOM!</div>', got:\n%s", output)
		}
	})
}
