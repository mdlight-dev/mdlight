// Package watch wraps fsnotify with debouncing for MDLight's file watcher.
//
// Design constraints (from LDD §5):
//
//  1. Watch the parent directory, not the file itself. On Linux (inotify),
//     a watch on a file is silently removed when that file is renamed away.
//     Most editors (vim, VS Code, etc.) save via temp-file + rename, so a
//     file-level watch dies on the very first save and never fires again.
//     Watching the directory and filtering by filename sidesteps this entirely.
//
//  2. Debounce ~150ms. A single logical save from most editors fires a burst
//     of RENAME + CREATE + WRITE events. Reacting to the first event would
//     attempt to read a partially-written or temporarily absent file. Waiting
//     for the burst to settle before calling onChange is the correct default.
//
//  3. Ignore CHMOD events. Some editors touch file permissions on save.
//     Only Write, Create, and Rename events on the target file trigger onChange.
//
//  4. The onChange callback is called from a time.AfterFunc goroutine. It must
//     be safe to call from any goroutine — runtime.EventsEmit satisfies this.
package watch

import (
	"path/filepath"
	"time"

	"github.com/fsnotify/fsnotify"
)

// Watcher watches a single file path and calls onChange after a quiet period
// following any Write, Create, or Rename event on that file.
type Watcher struct {
	watcher  *fsnotify.Watcher
	path     string        // filepath.Clean'd absolute path of the target file
	onChange func()        // called after debounce settles; runs in its own goroutine
	stop     chan struct{} // closed by Close to terminate the event loop
}

// New creates a Watcher for the file at path. onChange is called at most once
// per debounce window after filesystem activity on the target file.
//
// The caller is responsible for calling Close when the watcher is no longer
// needed (e.g., when a different file is opened, or on app shutdown).
func New(path string, debounce time.Duration, onChange func()) (*Watcher, error) {
	fw, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	// Watch the parent directory — not the file itself — so rename-based
	// saves (the default in vim, VS Code, and most editors on Linux) are
	// caught correctly. inotify removes a watch on the inode when that inode
	// is renamed; watching the directory uses a stable inode.
	dir := filepath.Dir(filepath.Clean(path))
	if err := fw.Add(dir); err != nil {
		fw.Close()
		return nil, err
	}

	w := &Watcher{
		watcher:  fw,
		path:     filepath.Clean(path),
		onChange: onChange,
		stop:     make(chan struct{}),
	}
	go w.loop(debounce)
	return w, nil
}

// Close stops the watcher and releases all resources. Safe to call more than
// once (subsequent calls are no-ops because the stop channel is already closed).
func (w *Watcher) Close() error {
	select {
	case <-w.stop:
		// Already closed.
	default:
		close(w.stop)
	}
	return w.watcher.Close()
}

// loop is the event processing goroutine. It reads fsnotify events, filters
// them to the target file and relevant operation types, and debounces bursts
// into a single onChange call.
func (w *Watcher) loop(debounce time.Duration) {
	var timer *time.Timer

	for {
		select {
		case event, ok := <-w.watcher.Events:
			if !ok {
				if timer != nil {
					timer.Stop()
				}
				return
			}

			// Filter: only react to our target file.
			if filepath.Clean(event.Name) != w.path {
				continue
			}

			// Filter: ignore CHMOD and REMOVE events.
			// Write  — direct in-place write (rare but possible)
			// Create — the replacement file appeared after a temp+rename save
			// Rename — the temp file was renamed over the original
			relevant := fsnotify.Write | fsnotify.Create | fsnotify.Rename
			if event.Op&relevant == 0 {
				continue
			}

			// Debounce: every relevant event resets the timer. onChange fires
			// only after the burst has been quiet for the debounce duration.
			if timer != nil {
				timer.Stop()
			}
			timer = time.AfterFunc(debounce, w.onChange)

		case _, ok := <-w.watcher.Errors:
			if !ok {
				return
			}
			// Swallow watcher errors — a transient inotify error should not
			// crash the application.
			continue

		case <-w.stop:
			if timer != nil {
				timer.Stop()
			}
			return
		}
	}
}
