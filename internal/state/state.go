// Package state provides XDG-compliant persistence for MDLight's
// per-session and per-installation state: recent files, window geometry,
// and other user preferences that survive between runs.
//
// Storage layout (XDG_CONFIG_HOME / XDG_DATA_HOME):
//   ~/.config/mdlight/        — themes (managed by theme package)
//   ~/.local/share/mdlight/   — recent-files list, settings
//
// XDG is preferred over a dotfile in the home directory because it keeps
// MDLight's data in the standard location that backup tools and desktop
// environments already know about.
package state

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
)

const (
	recentFilesMax = 10
	recentFilesName = "recent-files.json"
)

// recentFiles holds the persisted list of recently opened file paths.
type recentFiles struct {
	Paths []string `json:"paths"`
}

// dataDir returns the XDG data directory for MDLight.
// On Linux: ~/.local/share/mdlight/
// On macOS: ~/Library/Application Support/mdlight/
// On Windows: %AppData%/mdlight/
func dataDir() (string, error) {
	var base string

	switch runtime.GOOS {
	case "windows":
		base = os.Getenv("APPDATA")
		if base == "" {
			base = filepath.Join(os.Getenv("USERPROFILE"), "AppData", "Roaming")
		}
	case "darwin":
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		base = filepath.Join(home, "Library", "Application Support")
	default:
		base = os.Getenv("XDG_DATA_HOME")
		if base == "" {
			home, err := os.UserHomeDir()
			if err != nil {
				return "", err
			}
			base = filepath.Join(home, ".local", "share")
		}
	}

	dir := filepath.Join(base, "mdlight")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	return dir, nil
}

// AddRecent appends path to the recent-files list, deduplicates, trims to
// recentFilesMax entries, and persists to disk.
func AddRecent(path string) error {
	abs, err := filepath.Abs(path)
	if err != nil {
		return err
	}

	paths := loadRecent()

	// Remove existing occurrence so the new one rises to the top
	filtered := make([]string, 0, len(paths))
	for _, p := range paths {
		if p != abs {
			filtered = append(filtered, p)
		}
	}

	// Prepend to front
	paths = append([]string{abs}, filtered...)

	// Trim
	if len(paths) > recentFilesMax {
		paths = paths[:recentFilesMax]
	}

	return saveRecent(paths)
}

// RecentFiles returns the list of recently opened file paths, most recent first.
func RecentFiles() []string {
	return loadRecent()
}

// ClearRecent removes all recent files entries.
func ClearRecent() error {
	return saveRecent(nil)
}

func loadRecent() []string {
	dir, err := dataDir()
	if err != nil {
		return nil
	}

	data, err := os.ReadFile(filepath.Join(dir, recentFilesName))
	if err != nil {
		return nil
	}

	var rf recentFiles
	if err := json.Unmarshal(data, &rf); err != nil {
		return nil
	}

	return rf.Paths
}

func saveRecent(paths []string) error {
	dir, err := dataDir()
	if err != nil {
		return err
	}

	rf := recentFiles{Paths: paths}
	data, err := json.Marshal(rf)
	if err != nil {
		return err
	}

	return os.WriteFile(filepath.Join(dir, recentFilesName), data, 0644)
}
