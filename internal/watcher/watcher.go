package watcher

import (
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"punctpolish/internal/config"
	"punctpolish/internal/fileutil"
	"punctpolish/internal/processor"
	"punctpolish/internal/scanner"
)

// Watcher wraps fsnotify and adds recursive watching, extension filtering,
// debouncing, and self-write suppression.
type Watcher struct {
	cfg    config.Config
	fw     *fsnotify.Watcher
	proc   *processor.Processor
	guard  *fileutil.WriteGuard
	filter *scanner.Filter

	// debounce state: path → pending timer
	dmu     sync.Mutex
	pending map[string]*time.Timer
}

// New creates a Watcher ready to watch rootDir.
func New(cfg config.Config, proc *processor.Processor, guard *fileutil.WriteGuard) (*Watcher, error) {
	fw, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	return &Watcher{
		cfg:     cfg,
		fw:      fw,
		proc:    proc,
		guard:   guard,
		filter:  scanner.NewFilter(cfg.Extensions, cfg.IgnoreDirs),
		pending: make(map[string]*time.Timer),
	}, nil
}

// AddDir recursively registers dir and all its subdirectories with fsnotify.
func (w *Watcher) AddDir(dir string) error {
	return filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			slog.Warn("cannot access path during walk", "path", path, "error", err)
			return nil // keep walking
		}
		if !d.IsDir() {
			return nil
		}
		if w.filter.IgnoreDir(filepath.Base(path)) {
			slog.Debug("ignoring directory", "path", path)
			return filepath.SkipDir
		}
		if err := w.fw.Add(path); err != nil {
			slog.Warn("cannot watch directory", "path", path, "error", err)
			return nil
		}
		slog.Info("watching directory", "path", path)
		return nil
	})
}

// ScanAndProcess walks dir and immediately processes all matching files.
// Used only when --scan-on-start is set.
func (w *Watcher) ScanAndProcess(dir string) {
	scanner.Walk(dir, w.filter, w.proc)
}

// Run starts the event loop. It blocks until done is closed.
func (w *Watcher) Run(done <-chan struct{}) {
	for {
		select {
		case <-done:
			return

		case event, ok := <-w.fw.Events:
			if !ok {
				return
			}
			w.handleEvent(event)

		case err, ok := <-w.fw.Errors:
			if !ok {
				return
			}
			slog.Error("fsnotify error", "error", err)
		}
	}
}

// Close releases fsnotify resources.
func (w *Watcher) Close() error {
	return w.fw.Close()
}

// handleEvent dispatches a single fsnotify event.
func (w *Watcher) handleEvent(event fsnotify.Event) {
	path := event.Name

	// A new directory was created; register it immediately.
	if event.Has(fsnotify.Create) {
		if info, err := os.Stat(path); err == nil && info.IsDir() {
			if !w.filter.IgnoreDir(filepath.Base(path)) {
				if err := w.AddDir(path); err != nil {
					slog.Warn("cannot add new directory", "path", path, "error", err)
				}
			}
			return
		}
	}

	// We only care about create, write, and rename events on target files.
	relevant := event.Has(fsnotify.Create) || event.Has(fsnotify.Write) || event.Has(fsnotify.Rename)
	if !relevant {
		if event.Has(fsnotify.Remove) {
			slog.Debug("file removed", "path", path)
		}
		return
	}

	if !w.filter.MatchesExt(path) {
		return
	}

	// Skip events caused by our own writes.
	if w.guard.IsSelfWrite(path) {
		slog.Debug("skipping self-triggered event", "path", path)
		return
	}

	slog.Debug("file changed", "path", path, "op", event.Op.String())
	w.scheduleProcess(path)
}

// scheduleProcess debounces processing: if another event for the same path
// arrives before the timer fires, the timer is reset.
func (w *Watcher) scheduleProcess(path string) {
	w.dmu.Lock()
	defer w.dmu.Unlock()

	if t, ok := w.pending[path]; ok {
		t.Reset(w.cfg.Debounce)
		return
	}

	w.pending[path] = time.AfterFunc(w.cfg.Debounce, func() {
		w.dmu.Lock()
		delete(w.pending, path)
		w.dmu.Unlock()

		slog.Info("file changed", "path", path)
		w.proc.Process(path) //nolint:errcheck // watch loop: logged inside Process
	})
}

