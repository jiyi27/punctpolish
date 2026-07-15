package scanner

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

var directoryExtensions = map[string]struct{}{
	".md":  {},
	".txt": {},
}

// Handler processes a single file.
type Handler interface {
	Process(path string) (changed bool, err error)
}

// Walk recursively processes Markdown and plain-text files under dir. A
// directory target deliberately uses a fixed extension allowlist, while a
// single file is validated by the processor's text-content detection instead.
func Walk(dir string, h Handler) {
	_ = filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			slog.Warn("cannot access path", "path", path, "error", err)
			return nil
		}
		if d.IsDir() || !matchesDirectoryExtension(path) {
			return nil
		}
		if _, err := h.Process(path); err != nil {
			slog.Warn("failed to process file", "path", path, "error", err)
		}
		return nil
	})
}

func matchesDirectoryExtension(path string) bool {
	_, ok := directoryExtensions[strings.ToLower(filepath.Ext(path))]
	return ok
}
