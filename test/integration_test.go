//go:build integration

// Package test contains end-to-end integration tests for punctpolish.
//
// These tests intentionally stay small and opinionated:
//   - one test for the core watcher flow
//   - one test to prove startup does not rewrite existing files
//   - one test for --scan-on-start, because it changes that default behavior
//
// Run with:
//
//	go test -tags integration -v ./test/
package test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"punctpolish/internal/processor"
)

const (
	testDebounce = "200ms"
	grace        = 700 * time.Millisecond
	binaryName   = "/tmp/punctpolish-integration-test"
)

type fileSnapshot struct {
	content string
	modTime time.Time
}

// TestMain builds the binary once before running all integration tests.
func TestMain(m *testing.M) {
	cmd := exec.Command("go", "build", "-o", binaryName, "../cmd/punctpolish")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		panic("failed to build punctpolish: " + err.Error())
	}
	defer os.Remove(binaryName)

	os.Exit(m.Run())
}

func startWatcher(t *testing.T, dir string, extraArgs ...string) func() {
	t.Helper()

	args := append([]string{"--dir", dir, "--debounce", testDebounce}, extraArgs...)
	cmd := exec.Command(binaryName, args...)
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		t.Fatalf("cannot start watcher: %v", err)
	}

	time.Sleep(300 * time.Millisecond)
	return func() { _ = cmd.Process.Kill() }
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()

	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("cannot read %s: %v", path, err)
	}
	return string(b)
}

func snapshotFile(t *testing.T, path string) fileSnapshot {
	t.Helper()

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("cannot stat %s: %v", path, err)
	}

	return fileSnapshot{
		content: readFile(t, path),
		modTime: info.ModTime(),
	}
}

func assertFileUnchanged(t *testing.T, path string, before fileSnapshot) {
	t.Helper()

	after := snapshotFile(t, path)
	if after.content != before.content {
		t.Fatalf("expected %s content to stay unchanged\ngot:  %q\nwant: %q", path, after.content, before.content)
	}
	if !after.modTime.Equal(before.modTime) {
		t.Fatalf("expected %s mod time to stay unchanged\ngot:  %v\nwant: %v", path, after.modTime, before.modTime)
	}
}

func waitForFileContent(t *testing.T, path string, want string) {
	t.Helper()

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if got := readFile(t, path); got == want {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}

	got := readFile(t, path)
	t.Fatalf("file did not reach expected content in time\ngot:  %q\nwant: %q", got, want)
}

func TestWatcher_ModifiesOnlyTheFileThatChanged(t *testing.T) {
	dir := t.TempDir()

	targetPath := filepath.Join(dir, "target.md")
	targetOriginal := "# note\n\nAlready clean.\n"
	writeFile(t, targetPath, targetOriginal)

	untouchedPath := filepath.Join(dir, "untouched.md")
	untouchedOriginal := "苹果、香蕉、橙子。\n"
	writeFile(t, untouchedPath, untouchedOriginal)

	untouchedBefore := snapshotFile(t, untouchedPath)

	stop := startWatcher(t, dir)
	defer stop()

	targetUpdated := "# note\n\n结果：成功！请联系admin@example.com。\n新增ERP系统说明。\n"
	writeFile(t, targetPath, targetUpdated)

	want := processor.NormalizeText(targetUpdated)
	waitForFileContent(t, targetPath, want)

	assertFileUnchanged(t, untouchedPath, untouchedBefore)
}

func TestWatcher_DoesNotTouchExistingFilesOnStart(t *testing.T) {
	dir := t.TempDir()

	firstPath := filepath.Join(dir, "first.md")
	secondPath := filepath.Join(dir, "second.md")

	writeFile(t, firstPath, "苹果、香蕉、橙子。\n")
	writeFile(t, secondPath, "ERP系统和JSON数据以及这个Agent。\n")

	firstBefore := snapshotFile(t, firstPath)
	secondBefore := snapshotFile(t, secondPath)

	stop := startWatcher(t, dir)
	defer stop()

	time.Sleep(grace)

	assertFileUnchanged(t, firstPath, firstBefore)
	assertFileUnchanged(t, secondPath, secondBefore)
}

func TestWatcher_ScanOnStartProcessesExistingFiles(t *testing.T) {
	dir := t.TempDir()

	path := filepath.Join(dir, "pre.md")
	original := "苹果、香蕉、橙子。\n"
	writeFile(t, path, original)

	stop := startWatcher(t, dir, "--scan-on-start")
	defer stop()

	time.Sleep(grace)

	got := readFile(t, path)
	want := processor.NormalizeText(original)
	if got != want {
		t.Fatalf("expected --scan-on-start to normalize existing file\ngot:  %q\nwant: %q", got, want)
	}
}
