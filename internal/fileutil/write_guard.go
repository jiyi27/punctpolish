package fileutil

import (
	"sync"
	"time"
)

// WriteGuard tracks files recently written by the program itself so that the
// resulting fsnotify events can be suppressed and an infinite processing loop
// is avoided.
type WriteGuard struct {
	mu      sync.Mutex
	entries map[string]time.Time
	window  time.Duration
}

// NewWriteGuard creates a WriteGuard that ignores self-triggered events that
// arrive within window after a write.
func NewWriteGuard(window time.Duration) *WriteGuard {
	return &WriteGuard{
		entries: make(map[string]time.Time),
		window:  window,
	}
}

// Mark records that the program is about to write path.
// Call this immediately before writing the file.
func (g *WriteGuard) Mark(path string) {
	g.mu.Lock()
	g.entries[path] = time.Now()
	g.mu.Unlock()
}

// IsSelfWrite returns true if the event for path arrived within the guard
// window of the last Mark call, indicating the event was caused by the
// program itself.
func (g *WriteGuard) IsSelfWrite(path string) bool {
	g.mu.Lock()
	t, ok := g.entries[path]
	g.mu.Unlock()

	if !ok {
		return false
	}
	if time.Since(t) < g.window {
		return true
	}
	// Entry is stale; clean it up.
	g.mu.Lock()
	delete(g.entries, path)
	g.mu.Unlock()
	return false
}
