package processor

import (
	"log/slog"
	"os"

	"punctpolish/internal/fileutil"
)

const DefaultMaxFileSize = 10 * 1024 * 1024 // 10 MB

// Processor reads a file, normalizes its text content, and writes it back.
type Processor struct {
	maxFileSize int64
}

// New creates a Processor.
func New(maxFileSize int64) *Processor {
	return &Processor{
		maxFileSize: maxFileSize,
	}
}

// Process applies text normalization to the file at path.
// It returns whether the file was changed and any error encountered.
// The caller decides how to handle errors.
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

	if err := os.WriteFile(path, []byte(normalized), info.Mode()); err != nil {
		slog.Error("cannot write file", "path", path, "error", err)
		return false, err
	}

	slog.Info("file normalized", "path", path)
	return true, nil
}
