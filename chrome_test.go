package main

import (
	"os"
	"testing"
)

func TestFindChrome(t *testing.T) {
	bin := FindChrome()
	if bin == "" {
		t.Fatal("FindChrome() returned empty string; expected valid Chrome/Chromium executable path")
	}

	if _, err := os.Stat(bin); err != nil {
		t.Fatalf("FindChrome() returned %q which does not exist: %v", bin, err)
	}
}

func TestChromeInstallInstructions(t *testing.T) {
	if len(ChromeInstallInstructions) < 10 {
		t.Fatal("ChromeInstallInstructions appears to be corrupt")
	}
}
