package main

import (
	_ "embed"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

// FindChrome searches the host system for an installed Chrome or Chromium executable.
// If none is found, it returns an empty string.
func FindChrome() string {
	if env := os.Getenv("CHROME_BIN"); env != "" {
		if path, err := exec.LookPath(env); err == nil {
			return path
		}
	}

	var locations []string
	switch runtime.GOOS {
	case "darwin":
		locations = []string{
			"/Applications/Chromium.app/Contents/MacOS/Chromium",
			"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
			"chromium",
			"google-chrome",
			"chrome",
		}
	case "windows":
		locations = []string{
			"chrome",
			"chrome.exe",
			`C:\Program Files\Google\Chrome\Application\chrome.exe`,
			`C:\Program Files (x86)\Google\Chrome\Application\chrome.exe`,
		}
		if pf := os.Getenv("ProgramFiles"); pf != "" {
			locations = append(locations, filepath.Join(pf, "Google", "Chrome", "Application", "chrome.exe"))
		}
		if pfx86 := os.Getenv("ProgramFiles(x86)"); pfx86 != "" {
			locations = append(locations, filepath.Join(pfx86, "Google", "Chrome", "Application", "chrome.exe"))
		}
		if local := os.Getenv("LocalAppData"); local != "" {
			locations = append(locations, filepath.Join(local, "Google", "Chrome", "Application", "chrome.exe"))
		}
		locations = append(locations, "chromium", "chromium.exe")
	default:
		locations = []string{
			"chromium",
			"google-chrome",
			"chromium-browser",
			"chrome",
			"google-chrome-stable",
			"headless_shell",
			"headless-shell",
		}
	}

	for _, path := range locations {
		if found, err := exec.LookPath(path); err == nil {
			return found
		}
	}

	return ""
}

// ChromeInstallInstructions returns copy-pasteable commands for installing Chromium
// or Chrome across portable tools and system package managers.
//
//go:embed howto_install_chrome.txt
var ChromeInstallInstructions string
