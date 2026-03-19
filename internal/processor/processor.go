package processor

import (
	"log/slog"
	"os"
	"punctpolish/internal/fileutil"
)

// Processor reads a file, normalizes its text content, and writes it back.
// It relies on a WriteGuard to mark files it writes so the watcher can skip
// the resulting self-triggered events.
type Processor struct {
	guard       *fileutil.WriteGuard
	maxFileSize int64
	dryRun      bool
}

// New creates a Processor.
func New(guard *fileutil.WriteGuard, maxFileSize int64, dryRun bool) *Processor {
	return &Processor{
		guard:       guard,
		maxFileSize: maxFileSize,
		dryRun:      dryRun,
	}
}

// Process applies text normalization to the file at path.
// Errors are logged and never propagated to the caller, so a single bad file
// cannot stop the watch loop.
func (p *Processor) Process(path string) {
	info, err := os.Stat(path)
	if err != nil {
		slog.Error("cannot stat file", "path", path, "error", err)
		return
	}

	if info.Size() > p.maxFileSize {
		slog.Warn("file exceeds size limit, skipping",
			"path", path,
			"size", info.Size(),
			"limit", p.maxFileSize,
		)
		return
	}

	ok, err := fileutil.IsTextFile(path)
	if err != nil {
		slog.Error("cannot read file for text detection", "path", path, "error", err)
		return
	}
	if !ok {
		slog.Warn("non-text file skipped", "path", path, "error", "binary content detected")
		return
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		slog.Error("cannot read file", "path", path, "error", err)
		return
	}

	original := string(raw)
	normalized := NormalizeText(original)

	if normalized == original {
		slog.Debug("file unchanged after normalization", "path", path)
		return
	}

	if p.dryRun {
		slog.Info("dry-run: file would be normalized", "path", path)
		return
	}

	// Mark before writing so the resulting fsnotify event is suppressed.
	p.guard.Mark(path)

	if err := os.WriteFile(path, []byte(normalized), info.Mode()); err != nil {
		slog.Error("cannot write file", "path", path, "error", err)
		return
	}

	slog.Info("file normalized", "path", path)
}
