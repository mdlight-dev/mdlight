package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"mdlight/internal/render"
	"mdlight/internal/theme"
	"mdlight/internal/watch"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// startTime is set at process start for --bench timing measurements.
var startTime = time.Now()

// App is the Wails application struct. Its exported methods are bound to the
// frontend and callable from Svelte via the generated wailsjs bindings.
//
// Only the methods listed in §4 of the LDD are exposed across this boundary —
// nothing wildcard-bound, nothing reflection-based.
type App struct {
	ctx          context.Context
	startupFile  string
	startupTheme string
	bench        bool              // if true, print timing and exit after first load
	watcher      *watch.Watcher
}

// NewApp creates the App. filePath and themeName are parsed from os.Args in
// main.go before wails.Run, so they are available before the webview starts.
func NewApp(filePath, themeName string, bench bool) *App {
	return &App{
		startupFile:  filePath,
		startupTheme: themeName,
		bench:        bench,
	}
}

// startup is called by Wails when the application starts. Stores the
// application context, which is needed for EventsEmit and native dialogs.
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	if a.bench {
		println("[bench] startup:", time.Since(startTime).Round(time.Millisecond).String())
	}
}

// shutdown is called by Wails after the window closes. Cleans up the watcher
// so there are no goroutine leaks on exit.
func (a *App) shutdown(ctx context.Context) {
	if a.watcher != nil {
		a.watcher.Close()
		a.watcher = nil
	}
}

// DocumentPayload is re-exported at the package level so Wails generates
// the correct TypeScript bindings from app.go. The struct itself is defined
// in internal/render.
type DocumentPayload = render.DocumentPayload

// FrontMatter is similarly re-exported for Wails binding generation.
type FrontMatter = render.FrontMatter

// ThemeInfo is re-exported from internal/theme for Wails binding generation.
type ThemeInfo = theme.Info

// ── Startup accessors ────────────────────────────────────────────────────────

// GetStartupFile returns the file path parsed from os.Args before wails.Run.
// The frontend calls this in onMount — no startup-event race, the binding is
// always available by the time onMount runs.
func (a *App) GetStartupFile() string { return a.startupFile }

// GetStartupTheme returns the --theme flag value parsed from os.Args.
func (a *App) GetStartupTheme() string { return a.startupTheme }

// ── File operations ──────────────────────────────────────────────────────────

// OpenFile reads the file at path, runs the full rendering pipeline, sets the
// window title, starts the file watcher, and returns a DocumentPayload.
//
// Error handling: only filesystem failures (file not found, permission denied)
// produce an error. Rendering failures degrade gracefully inside render.Render.
func (a *App) OpenFile(path string) (DocumentPayload, error) {
	src, err := os.ReadFile(path)
	if err != nil {
		return DocumentPayload{}, err
	}

	payload := render.Render(src)
	payload.HTML = render.RewriteImages(payload.HTML, filepath.Dir(path))

	if a.ctx != nil {
		wailsruntime.WindowSetTitle(a.ctx, filepath.Base(path))
	}

	a.startWatching(path)

	if a.bench {
		println("[bench] open-file duration:", time.Since(startTime).Round(time.Millisecond).String())
		// Schedule quit on next tick so the response reaches the frontend first
		go func() {
			time.Sleep(50 * time.Millisecond)
			wailsruntime.Quit(a.ctx)
		}()
	}

	return payload, nil
}

// SaveFile writes rawMarkdown to path atomically: it writes to a temp file in
// the same directory first, then renames over the original. A crash mid-save
// cannot leave a half-written file.
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
		return fmt.Errorf("SaveFile: close temp: %w", err)
	}

	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("SaveFile: rename: %w", err)
	}

	return nil
}

// PickFile opens a native file picker dialog filtered to Markdown files.
// Returns the selected path, or an empty string if the user cancels.
func (a *App) PickFile() (string, error) {
	path, err := wailsruntime.OpenFileDialog(a.ctx, wailsruntime.OpenDialogOptions{
		Title: "Open Markdown file",
		Filters: []wailsruntime.FileFilter{
			{DisplayName: "Markdown files", Pattern: "*.md;*.markdown"},
			{DisplayName: "All files", Pattern: "*"},
		},
	})
	if err != nil {
		return "", err
	}
	return path, nil
}

// ── Theme operations ─────────────────────────────────────────────────────────

// ResolveTheme returns the full CSS text for the named theme.
// Resolution order: XDG user dir → embedded built-ins → error with available names.
func (a *App) ResolveTheme(name string) (string, error) {
	css, err := theme.Resolve(name)
	if err != nil {
		// Enrich the error with the list of available theme names so the user
		// knows what to pass to --theme. theme.Resolve only reports the lookup
		// failure; we add the available list here where we have the full context.
		if available, listErr := theme.List(); listErr == nil {
			names := make([]string, 0, len(available))
			for _, t := range available {
				names = append(names, t.Name)
			}
			sort.Strings(names)
			return "", fmt.Errorf("%w — available themes: %s", err, strings.Join(names, ", "))
		}
		return "", err
	}
	return css, nil
}

// ListThemes returns all available themes: built-ins first, then user themes.
func (a *App) ListThemes() ([]ThemeInfo, error) {
	return theme.List()
}

// ── Recent files (v1.0 stub) ─────────────────────────────────────────────────

// GetRecentFiles returns the list of recently opened file paths.
// Stubbed for v1.0 (internal/state not yet built).
func (a *App) GetRecentFiles() ([]string, error) {
	return []string{}, nil
}

// ── Internal: file watcher ───────────────────────────────────────────────────

// startWatching replaces any existing watcher with one for path.
// Called from OpenFile after a successful render.
//
// Not a Wails binding — the frontend never calls this directly. The watcher
// communicates back to the frontend via the "file:changed" event.
func (a *App) startWatching(path string) {
	if a.watcher != nil {
		a.watcher.Close()
		a.watcher = nil
	}

	absPath, err := filepath.Abs(path)
	if err != nil {
		// Non-fatal: auto-reload won't work for this file, but the document
		// still opened and rendered correctly.
		return
	}

	w, err := watch.New(absPath, 150*time.Millisecond, func() {
		// This callback runs in a time.AfterFunc goroutine.
		// EventsEmit is safe to call from any goroutine.
		wailsruntime.EventsEmit(a.ctx, "file:changed", absPath)
	})
	if err != nil {
		// Non-fatal: watcher failing means no auto-reload, not a crash.
		return
	}

	a.watcher = w
}
