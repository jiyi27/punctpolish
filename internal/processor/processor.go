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
// It returns whether the file was changed and any error encountered.
// The caller decides how to handle errors; the watch loop logs and ignores them.
func (p *Processor) Process(path string) (changed bool, err error) {
	info, err := os.Stat(path)
	if err != nil {
		slog.Error("cannot stat file", "path", path, "error", err)
		return false, err
	}

	if info.Size() > p.maxFileSize {
		slog.Warn("file exceeds size limit, skipping",
			"path", path,
			"size", info.Size(),
			"limit", p.maxFileSize,
		)
		return false, nil
	}

	ok, err := fileutil.IsTextFile(path)
	if err != nil {
		slog.Error("cannot read file for text detection", "path", path, "error", err)
		return false, err
	}
	if !ok {
		slog.Warn("non-text file skipped", "path", path, "error", "binary content detected")
		return false, nil
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		slog.Error("cannot read file", "path", path, "error", err)
		return false, err
	}

	original := string(raw)
	normalized := NormalizeText(original)

	if normalized == original {
		slog.Debug("file unchanged after normalization", "path", path)
		return false, nil
	}

	if p.dryRun {
		slog.Info("dry-run: file would be normalized", "path", path)
		return true, nil
	}

	// Mark before writing so the resulting fsnotify event is suppressed.
	p.guard.Mark(path)

	if err := os.WriteFile(path, []byte(normalized), info.Mode()); err != nil {
		slog.Error("cannot write file", "path", path, "error", err)
		return false, err
	}

	slog.Info("file normalized", "path", path)
	return true, nil
}
