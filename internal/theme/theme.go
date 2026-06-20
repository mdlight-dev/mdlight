// Package theme handles discovery and resolution of MDLight themes.
//
// Resolution order (from LDD §3):
//  1. ~/.config/mdlight/themes/<name>.css   (XDG user themes)
//  2. Embedded built-in themes              (compiled into the binary)
//  3. Error listing available names
//
// A theme file is pure CSS — a :root {} block defining the --md-* custom
// property contract, optionally followed by .chroma rules for syntax
// highlighting. No custom parser is needed; the Go side just reads and
// returns the CSS text, and the frontend injects it into a <style> tag.
package theme

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
)

// builtinFS holds the CSS files in internal/theme/builtin/.
// These are the shipped themes compiled into the binary.
//
//go:embed builtin/*.css
var builtinFS embed.FS

// Info describes a single available theme.
type Info struct {
	Name   string // e.g. "default-dark", "nord"
	Source string // "builtin" or "user"
}

// Resolve returns the full CSS text for the named theme.
// It follows the resolution order in LDD §3.
func Resolve(name string) (string, error) {
	if name == "" {
		return "", fmt.Errorf("theme name must not be empty")
	}

	// 1. XDG user themes: ~/.config/mdlight/themes/<name>.css
	userPath := userThemePath(name)
	if data, err := os.ReadFile(userPath); err == nil {
		return string(data), nil
	}

	// 2. Embedded built-ins.
	embedPath := "builtin/" + name + ".css"
	data, err := builtinFS.ReadFile(embedPath)
	if err == nil {
		return string(data), nil
	}

	// 3. Not found.
	return "", fmt.Errorf("theme %q not found (looked in %s and built-ins)", name, userThemeDir())
}

// List returns all available themes: built-ins first in defined order,
// then user themes alphabetically. Built-in order follows the order they
// appear in the builtin/ directory listing (alphabetical by filename).
func List() ([]Info, error) {
	var themes []Info

	// Embedded built-ins.
	entries, err := builtinFS.ReadDir("builtin")
	if err != nil {
		return nil, fmt.Errorf("theme.List: read embedded themes: %w", err)
	}
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".css") {
			name := strings.TrimSuffix(e.Name(), ".css")
			themes = append(themes, Info{Name: name, Source: "builtin"})
		}
	}

	// Ensure built‑in theme order is deterministic (alphabetical by name).
	sort.Slice(themes, func(i, j int) bool {
		return themes[i].Name < themes[j].Name
	})

	// User themes from XDG config dir.
	dir := userThemeDir()
	userEntries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			// No user theme dir yet — that's fine.
			return themes, nil
		}
		return themes, fmt.Errorf("theme.List: read user theme dir %s: %w", dir, err)
	}

	var userThemes []Info
	for _, e := range userEntries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".css") {
			name := strings.TrimSuffix(e.Name(), ".css")
			userThemes = append(userThemes, Info{Name: name, Source: "user"})
		}
	}
	sort.Slice(userThemes, func(i, j int) bool {
		return userThemes[i].Name < userThemes[j].Name
	})

	return append(themes, userThemes...), nil
}

// userThemeDir returns the XDG-compliant directory for user themes.
// On Linux/macOS this is ~/.config/mdlight/themes.
// On Windows this is %APPDATA%\mdlight\themes.
func userThemeDir() string {
	switch runtime.GOOS {
	case "windows":
		appData := os.Getenv("APPDATA")
		if appData == "" {
			appData = filepath.Join(os.Getenv("USERPROFILE"), "AppData", "Roaming")
		}
		return filepath.Join(appData, "mdlight", "themes")
	default:
		// XDG: prefer XDG_CONFIG_HOME, fall back to ~/.config
		cfgHome := os.Getenv("XDG_CONFIG_HOME")
		if cfgHome == "" {
			home, _ := os.UserHomeDir()
			cfgHome = filepath.Join(home, ".config")
		}
		return filepath.Join(cfgHome, "mdlight", "themes")
	}
}

// userThemePath returns the full path for a named user theme file.
func userThemePath(name string) string {
	return filepath.Join(userThemeDir(), name+".css")
}
