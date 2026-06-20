package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"mdlight/internal/render"
	"mdlight/internal/theme"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// App is the Wails application struct. Its exported methods are bound to the
// frontend and callable from Svelte via the generated wailsjs bindings.
//
// Only the methods listed in §4 of the LDD are exposed across this boundary —
// nothing wildcard-bound, nothing reflection-based.
type App struct {
	ctx          context.Context
	startupFile  string // resolved from CLI args before wails.Run; never changes
	startupTheme string // --theme flag value; empty means use default
}

// NewApp creates the App with the CLI-resolved file path and theme name.
// Both values are stored before wails.Run so the frontend can retrieve them
// via GetStartupFile / GetStartupTheme without any timing race. (See the
// Option B discussion in HANDOFF_milestone4.md.)
func NewApp(filePath, themeName string) *App {
	return &App{
		startupFile:  filePath,
		startupTheme: themeName,
	}
}

// startup is called by Wails when the application starts. Receives the
// application context, which is needed for EventsEmit (file watcher, M6),
// native dialogs, and window title changes.
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

// ── Wails binding re-exports ────────────────────────────────────────────────
//
// DocumentPayload and FrontMatter are defined in internal/render. They are
// type-aliased here so Wails generates the correct TypeScript bindings from
// app.go — the binding surface lives in one file.

type DocumentPayload = render.DocumentPayload
type FrontMatter = render.FrontMatter

// ThemeInfo describes a single available theme for the ListThemes binding.
type ThemeInfo struct {
	Name   string // e.g. "default-dark", "nord"
	Source string // "builtin" or "user"
}

// ── Startup accessors ───────────────────────────────────────────────────────

// GetStartupFile returns the file path passed on the command line, or an
// empty string if none was given (which tells the frontend to open the picker).
func (a *App) GetStartupFile() string {
	return a.startupFile
}

// GetStartupTheme returns the --theme flag value, or an empty string if the
// flag was not set (which tells the frontend to use the default theme).
func (a *App) GetStartupTheme() string {
	return a.startupTheme
}

// ── File operations ─────────────────────────────────────────────────────────

// OpenFile reads the file at path, runs the full rendering pipeline, and
// returns a DocumentPayload for the frontend to display.
//
// After a successful render the window title is updated to the bare filename
// so the OS taskbar and title bar are meaningful.
func (a *App) OpenFile(path string) (DocumentPayload, error) {
	src, err := os.ReadFile(path)
	if err != nil {
		return DocumentPayload{}, err
	}

	payload := render.Render(src)

	// Update the window title to the bare filename (no directory).
	// runtime.WindowSetTitle is a no-op if ctx is nil (shouldn't happen, but
	// defensive — OpenFile is only ever called after startup).
	if a.ctx != nil {
		runtime.WindowSetTitle(a.ctx, filepath.Base(path))
	}

	return payload, nil
}

// PickFile opens the native OS file picker filtered to Markdown files and
// returns the chosen path. Returns an empty string (not an error) if the user
// cancels the dialog.
func (a *App) PickFile() (string, error) {
	path, err := runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Open Markdown file",
		Filters: []runtime.FileFilter{
			{
				DisplayName: "Markdown files",
				Pattern:     "*.md;*.markdown;*.mdown;*.mkd",
			},
			{
				DisplayName: "All files",
				Pattern:     "*",
			},
		},
	})
	if err != nil {
		return "", err
	}
	// path is empty string when the user cancels — that's fine, not an error.
	return path, nil
}

// SaveFile writes rawMarkdown to path atomically: it writes to a temp file in
// the same directory first, then renames over the original. A crash mid-save
// cannot leave a half-written file.
//
// The Wails binding exists now so the TypeScript types are generated; the
// frontend wires it in milestone 6 (file watcher + edit mode).
func (a *App) SaveFile(path string, rawMarkdown string) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".mdlight-save-*")
	if err != nil {
		return fmt.Errorf("SaveFile: create temp: %w", err)
	}
	tmpPath := tmp.Name()

	if _, err := tmp.WriteString(rawMarkdown); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("SaveFile: write: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("SaveFile: close: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("SaveFile: rename: %w", err)
	}
	return nil
}

// ── Theme operations ─────────────────────────────────────────────────────────

// ResolveTheme returns the full CSS text for a named theme. Resolution order
// (from LDD §3):
//  1. ~/.config/mdlight/themes/<name>.css
//  2. Embedded built-in themes
//  3. Error listing available names
func (a *App) ResolveTheme(name string) (string, error) {
	css, err := theme.Resolve(name)
	if err != nil {
		// Enrich the error with the list of known themes so the user's
		// --theme typo produces a useful message.
		available, listErr := theme.List()
		if listErr != nil || len(available) == 0 {
			return "", err
		}
		names := make([]string, len(available))
		for i, t := range available {
			names[i] = t.Name
		}
		sort.Strings(names)
		return "", fmt.Errorf("%w\n\nAvailable themes: %v", err, names)
	}
	return css, nil
}

// ListThemes returns metadata for every theme the user can select, in display
// order: built-ins first, then user themes alphabetically.
func (a *App) ListThemes() ([]ThemeInfo, error) {
	raw, err := theme.List()
	if err != nil {
		return nil, err
	}
	out := make([]ThemeInfo, len(raw))
	for i, t := range raw {
		out[i] = ThemeInfo{Name: t.Name, Source: t.Source}
	}
	return out, nil
}

// GetRecentFiles returns the list of recently opened file paths, most recent
// first. Implemented in milestone 5 (recent files list). The binding is
// defined here so the TypeScript types are generated now.
func (a *App) GetRecentFiles() ([]string, error) {
	// TODO(milestone-5): implement via internal/state
	return nil, nil
}
