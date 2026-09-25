// Package filewatcher provides a generic, single-file watcher that monitors
// a file for changes (edits, copies, moves) and pushes parsed content into
// a typed channel using Go generics.
//
// Internally it uses fsnotify for kernel-level inotify events, so there is
// no polling — the watcher sleeps until the OS delivers a notification.
//
// Usage:
//
//	type Config struct { Port int `json:"port"` }
//
//	cancel, ch, err := filewatcher.New[Config]("/etc/app/config.json",
//	    func(data []byte) (Config, error) {
//	        var c Config
//	        return c, json.Unmarshal(data, &c)
//	    },
//	)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer cancel()
//
//	for cfg := range ch {
//	    fmt.Println("new config:", cfg)
//	}
package watcher

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/fsnotify/fsnotify"
)

// ParseFunc is a user-supplied function that converts raw file bytes into
// a value of type T. Return a non-nil error to skip pushing to the channel.
type ParseFunc[T any] func(data []byte) (T, error)

// CancelFunc stops the watcher and closes the output channel.
// It is safe to call more than once.
type CancelFunc func()

// New creates a new file watcher for the file at filePath.
//
// Parameters:
//   - filePath  : absolute or relative path to the file to watch.
//   - parse     : function that converts raw bytes → T.
//   - bufSize   : capacity of the returned channel (0 = unbuffered).
//
// Returns:
//   - CancelFunc : call this to stop watching and close the channel.
//   - <-chan T   : receive parsed values whenever the file changes.
//   - error      : non-nil if the watcher cannot be initialised.
//
// The watcher monitors the *parent directory* of the file (not the file
// itself) so that atomic replacements via `mv` / `cp` are also detected.
// Only events that concern the target filename are forwarded.
func New[T any](filePath string, parse ParseFunc[T], bufSize int) (CancelFunc, <-chan T, error) {
	// ── Resolve the absolute path so directory derivation is reliable ──
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		return nil, nil, fmt.Errorf("filewatcher: cannot resolve path %q: %w", filePath, err)
	}

	// cleaning the filepath first before use.
	absPath = filepath.Clean(absPath)

	// The directory that contains the target file.
	dir := filepath.Dir(absPath)
	// Just the filename component used for event filtering.
	targetName := filepath.Base(absPath)

	// ── Create the underlying fsnotify watcher ──────────────────────────
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, nil, fmt.Errorf("filewatcher: cannot create fsnotify watcher: %w", err)
	}

	// Watch the parent directory so that mv/cp (which change the inode)
	// are also captured via Create / Rename events on the directory entry.
	if err := watcher.Add(dir); err != nil {
		_ = watcher.Close()
		return nil, nil, fmt.Errorf("filewatcher: cannot watch directory %q: %w", dir, err)
	}

	// ── Output channel ──────────────────────────────────────────────────
	ch := make(chan T, bufSize)

	// done is closed by the cancel function to signal the goroutine to stop.
	done := make(chan struct{})

	// ── cancel is idempotent thanks to the sync.Once pattern ───────────
	var cancelled bool
	cancel := func() {
		if cancelled {
			return
		}
		cancelled = true
		close(done)         // signal goroutine
		_ = watcher.Close() // unblock watcher.Events / watcher.Errors
	}

	// ── Helper: read the file and push a parsed value into ch ──────────
	// A small debounce (100 ms) is applied so that rapid successive events
	// (e.g. editor write + chmod) collapse into a single read.
	pushIfChanged := func() {
		// Brief sleep to let the writer finish flushing.
		time.Sleep(100 * time.Millisecond)

		raw, err := os.ReadFile(absPath)
		if err != nil {
			// File may have been temporarily absent during an atomic swap.
			return
		}

		value, err := parse(raw)
		if err != nil {
			// Parsing failed — do not push a potentially corrupt value.
			return
		}

		// Non-blocking send: if the consumer is slow we drop the event
		// rather than blocking the watcher goroutine.
		select {
		case ch <- value:
		case <-done:
		}
	}

	// ── Background goroutine ────────────────────────────────────────────
	go func() {
		defer close(ch) // signal consumers that no more values will arrive

		for {
			select {
			case <-done:
				// cancel() was called — exit cleanly.
				return

			case event, ok := <-watcher.Events:
				if !ok {
					// fsnotify closed its channel (watcher.Close was called).
					return
				}

				// Filter: only react to events on our target file.
				if filepath.Base(event.Name) != targetName {
					continue
				}

				// React to:
				//   Write  — in-place edit (vim, echo >>)
				//   Create — cp / mv into the directory (new inode)
				//   Rename — some editors rename a temp file into place
				//   Chmod  — some tools touch permissions after writing
				if event.Has(fsnotify.Write) ||
					event.Has(fsnotify.Create) ||
					event.Has(fsnotify.Rename) ||
					event.Has(fsnotify.Chmod) {
					pushIfChanged()
				}

			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				// Log or handle watcher-level errors here if needed.
				_ = err
			}
		}
	}()

	return cancel, ch, nil
}
