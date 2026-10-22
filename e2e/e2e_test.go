package e2e

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestE2E(t *testing.T) {
	tmpDir := t.TempDir()
	execBin := filepath.Join(tmpDir, "go-wasm-exec")

	buildCmd := exec.Command("go", "build", "-o", execBin, "..")
	buildCmd.Stdout = os.Stdout
	buildCmd.Stderr = os.Stderr
	if err := buildCmd.Run(); err != nil {
		t.Fatalf("Failed to build go-wasm-exec: %v", err)
	}

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
}
