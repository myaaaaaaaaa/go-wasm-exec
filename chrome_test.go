package main

import (
	"os/exec"
	"testing"
)

func TestFindChrome(t *testing.T) {
	bin := FindChrome()
	if bin == "" {
		t.Fatal("FindChrome() returned empty string; expected valid Chrome/Chromium executable path")
	}

	if _, err := exec.LookPath(bin); err != nil {
		t.Fatalf("FindChrome() returned %q which is not executable: %v", bin, err)
	}
}
