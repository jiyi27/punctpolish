package scanner

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

// Handler processes a single file.
type Handler interface {
	Process(path string) (changed bool, err error)
}

// Filter holds the extension and ignore-dir sets used to decide which files
// and directories to visit.
type Filter struct {
	extSet    map[string]struct{}
	ignoreSet map[string]struct{}
}

// NewFilter builds a Filter from the configured extensions and ignored
// directory names. Extensions should include the leading dot (e.g. ".md").
func NewFilter(extensions, ignoreDirs []string) *Filter {
	extSet := make(map[string]struct{}, len(extensions))
	for _, e := range extensions {
		extSet[strings.ToLower(e)] = struct{}{}
	}

	ignoreSet := make(map[string]struct{}, len(ignoreDirs))
	for _, d := range ignoreDirs {
		ignoreSet[d] = struct{}{}
	}

	return &Filter{extSet: extSet, ignoreSet: ignoreSet}
}

// MatchesExt reports whether path has one of the configured extensions.
func (f *Filter) MatchesExt(path string) bool {
	_, ok := f.extSet[strings.ToLower(filepath.Ext(path))]
	return ok
}

// IgnoreDir reports whether the directory name should be skipped entirely.
func (f *Filter) IgnoreDir(name string) bool {
	_, ok := f.ignoreSet[name]
	return ok
}

// Walk recursively visits dir, calling h.Process for every file that passes
// the filter. Errors from h.Process are logged but do not stop the walk.
func Walk(dir string, f *Filter, h Handler) {
	_ = filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			slog.Warn("scan: cannot access path", "path", path, "error", err)
			return nil
		}
		if d.IsDir() {
			if f.IgnoreDir(filepath.Base(path)) {
				return filepath.SkipDir
			}
			return nil
		}
		if f.MatchesExt(path) {
			if _, err := h.Process(path); err != nil {
				slog.Warn("scan: failed to process file", "path", path, "error", err)
			}
		}
		return nil
	})
}
