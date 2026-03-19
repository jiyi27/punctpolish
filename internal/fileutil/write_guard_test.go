package fileutil

import (
	"testing"
	"time"
)

func TestWriteGuard_BasicFlow(t *testing.T) {
	g := NewWriteGuard(200 * time.Millisecond)

	path := "/tmp/test.md"
	g.Mark(path)

	if !g.IsSelfWrite(path) {
		t.Error("expected IsSelfWrite to return true immediately after Mark")
	}
}

func TestWriteGuard_AfterWindow(t *testing.T) {
	g := NewWriteGuard(50 * time.Millisecond)

	path := "/tmp/test2.md"
	g.Mark(path)

	time.Sleep(100 * time.Millisecond)

	if g.IsSelfWrite(path) {
		t.Error("expected IsSelfWrite to return false after window expired")
	}
}

func TestWriteGuard_UnmarkedPath(t *testing.T) {
	g := NewWriteGuard(1 * time.Second)
	if g.IsSelfWrite("/tmp/never-marked.md") {
		t.Error("expected false for a path that was never marked")
	}
}
