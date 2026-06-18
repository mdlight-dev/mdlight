package main

import (
	"context"
	"os"

	"mdlight/internal/render"
)

// App is the Wails application struct. Its exported methods are bound to the
// frontend and callable from Svelte via the generated wailsjs bindings.
//
// Only the methods listed in §4 of the LDD are exposed across this boundary —
// nothing wildcard-bound, nothing reflection-based.
type App struct {
	ctx context.Context
}

// NewApp creates the App. Called by main.go before wails.Run.
func NewApp() *App {
	return &App{}
}

// startup is called by Wails when the application starts. Receives the
// application context, which is needed for EventsEmit (file watcher, v0.1
// task 6) and for opening native dialogs.
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

// DocumentPayload is re-exported at the package level so Wails generates
// the correct TypeScript bindings from app.go. The struct itself is defined
// in internal/render — this alias keeps the binding surface in one place
// while the implementation stays in the right package.
type DocumentPayload = render.DocumentPayload

// FrontMatter is similarly re-exported for Wails binding generation.
type FrontMatter = render.FrontMatter

// OpenFile reads the file at path, runs the full rendering pipeline, and
// returns a DocumentPayload for the frontend to display.
//
// Current state (milestone 2 of 8): path is hardcoded to a test file so we
// can prove the Go→Svelte data path before wiring real CLI argument parsing.
// Replace with the real path argument in milestone 4.
//
// Error handling: OpenFile returns (DocumentPayload, error) so the Wails
// binding surface is correct from the start. Rendering errors are already
// handled gracefully inside render.Render — this function only errors on
// filesystem failures (file not found, permission denied).
func (a *App) OpenFile(path string) (DocumentPayload, error) {
	// --- Milestone 2: hardcoded path for data-path verification ---
	// TODO(milestone-4): replace with the path argument from CLI args.
	if path == "" {
		path = "testdata/sample.md"
	}
	// --- end temporary ---

	src, err := os.ReadFile(path)
	if err != nil {
		return DocumentPayload{}, err
	}

	return render.Render(src), nil
}

// SaveFile writes rawMarkdown to path atomically: it writes to a temp file in
// the same directory first, then renames over the original. A crash mid-save
// cannot leave a half-written file. Defined now so the Wails binding exists;
// the frontend wires it in milestone 6 (file watcher conflict overlay).
func (a *App) SaveFile(path string, rawMarkdown string) error {
	// Atomic write: temp file in the same directory + rename.
	// os.CreateTemp with a dir argument of filepath.Dir(path) keeps the temp
	// file on the same filesystem as the destination, making the rename cheap
	// and atomic on POSIX systems.
	//
	// TODO: implement — placeholder for binding generation only.
	_ = path
	_ = rawMarkdown
	return nil
}
