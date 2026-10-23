package main

import (
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
const ChromeInstallInstructions = `
go-wasm-exec requires Chrome, Chromium, or a compatible headless browser.
Choose one of the commands below to install Chromium:

Portable & Package-Agnostic Tools (Recommended):
  * Playwright:
    npx playwright install chromium
    # or install with system dependencies on Linux:
    npx playwright install --with-deps chromium

  * Puppeteer / Chrome for Testing:
    npx @puppeteer/browsers install chrome@stable

  * Nix (declarative / portable package manager):
    nix profile add nixpkgs#chromium
    # or using nix-env:
    nix-env -iA nixpkgs.chromium

System Package Managers:
  * macOS (Homebrew):
    brew install --cask google-chrome
    # or open-source Chromium:
    brew install chromium

  * Windows (winget / Scoop / Chocolatey):
    winget install Google.Chrome
    scoop install chromium
    choco install googlechrome

  * Linux (Debian / Ubuntu):
    sudo apt-get update && sudo apt-get install -y chromium-browser

  * Linux (Fedora / RHEL):
    sudo dnf install -y chromium

  * Linux (Arch Linux):
    sudo pacman -S chromium

  * Linux (Alpine):
    apk add chromium

  * Linux (Snap / Flatpak):
    sudo snap install chromium
    flatpak install flathub org.chromium.Chromium

Environment Variable Override:
  If Chromium is installed at a custom or non-standard path, point to it directly:
  * Linux / macOS:
    export CHROME_BIN="/path/to/chromium"
  * Windows (PowerShell):
    $env:CHROME_BIN="C:\path\to\chrome.exe"
`
