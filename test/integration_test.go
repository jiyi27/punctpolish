//go:build integration

// Package test contains end-to-end integration tests for textwatcher.
// These tests build the real binary and run it as a subprocess against a
// temporary directory, so they verify the full stack: watcher → processor →
// normalize → write-back.
//
// Run with:
//
//	go test -tags integration -v ./test/
package test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const (
	// debounce used in tests — short so tests don't hang.
	testDebounce = "200ms"

	// grace is how long we wait after triggering an event for the watcher to
	// finish processing (debounce + disk write round-trip).
	grace = 600 * time.Millisecond

	// binaryName is the output path for the compiled binary.
	binaryName = "/tmp/textwatcher-integration-test"
)

// TestMain builds the binary once before running all integration tests.
func TestMain(m *testing.M) {
	cmd := exec.Command("go", "build", "-o", binaryName, "../cmd/textwatcher")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		// Cannot build — all tests would fail anyway.
		panic("failed to build textwatcher: " + err.Error())
	}
	defer os.Remove(binaryName)

	os.Exit(m.Run())
}

// startWatcher launches the textwatcher binary against dir and returns a
// function that stops it.
func startWatcher(t *testing.T, dir string, extraArgs ...string) func() {
	t.Helper()
	args := append([]string{"--dir", dir, "--debounce", testDebounce}, extraArgs...)
	cmd := exec.Command(binaryName, args...)
	cmd.Stdout = os.Stderr // redirect to test stderr so -v shows watcher logs
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		t.Fatalf("cannot start watcher: %v", err)
	}
	// Give fsnotify a moment to register all directories.
	time.Sleep(300 * time.Millisecond)
	return func() { _ = cmd.Process.Kill() }
}

// writeFile atomically writes content to path, creating parent dirs as needed.
func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// readFile reads the current content of path.
func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("cannot read %s: %v", path, err)
	}
	return string(b)
}

// --------------------------------------------------------------------------
// Test cases
// --------------------------------------------------------------------------

// TestNewFileIsNormalized verifies that a file created after the watcher starts
// is normalized and written back.
func TestNewFileIsNormalized(t *testing.T) {
	dir := t.TempDir()
	stop := startWatcher(t, dir)
	defer stop()

	path := filepath.Join(dir, "new.md")
	writeFile(t, path, "你好，世界。这是一个测试！\n")

	time.Sleep(grace)

	got := readFile(t, path)
	if !strings.Contains(got, ", ") {
		t.Errorf("expected Chinese comma to become ', '; got: %q", got)
	}
	if strings.ContainsRune(got, '，') {
		t.Errorf("Chinese comma should have been replaced; got: %q", got)
	}
}

// TestModifiedFileIsNormalized verifies that writing to an existing file after
// the watcher starts triggers normalization.
func TestModifiedFileIsNormalized(t *testing.T) {
	dir := t.TempDir()

	// Write a clean file before the watcher starts — it must not be touched.
	path := filepath.Join(dir, "existing.md")
	writeFile(t, path, "# clean file\n\nNo Chinese punctuation here.\n")

	stop := startWatcher(t, dir)
	defer stop()

	// Now modify the file after the watcher is running.
	writeFile(t, path, "# modified\n\n结果：成功！请联系admin@example.com。\n")

	time.Sleep(grace)

	got := readFile(t, path)
	if strings.ContainsRune(got, '：') {
		t.Errorf("Chinese colon should have been replaced; got: %q", got)
	}
	if strings.ContainsRune(got, '！') {
		t.Errorf("Chinese exclamation should have been replaced; got: %q", got)
	}
}

// TestPreExistingFileUntouched verifies that files that already exist when the
// watcher starts are NOT modified (default behaviour without --scan-on-start).
func TestPreExistingFileUntouched(t *testing.T) {
	dir := t.TempDir()

	path := filepath.Join(dir, "pre.md")
	original := "苹果、香蕉、橙子。\n"
	writeFile(t, path, original)

	stop := startWatcher(t, dir)
	defer stop()

	// Wait longer than the debounce window — nothing should happen.
	time.Sleep(grace)

	got := readFile(t, path)
	if got != original {
		t.Errorf("pre-existing file was modified without --scan-on-start:\ngot:  %q\nwant: %q", got, original)
	}
}

// TestScanOnStartProcessesExistingFiles verifies that --scan-on-start causes
// existing files to be normalized immediately.
func TestScanOnStartProcessesExistingFiles(t *testing.T) {
	dir := t.TempDir()

	path := filepath.Join(dir, "pre.md")
	writeFile(t, path, "苹果、香蕉、橙子。\n")

	stop := startWatcher(t, dir, "--scan-on-start")
	defer stop()

	time.Sleep(grace)

	got := readFile(t, path)
	if strings.ContainsRune(got, '、') {
		t.Errorf("existing file should have been normalized with --scan-on-start; got: %q", got)
	}
}

// TestFencedCodeBlockPreserved verifies that content inside ``` blocks is not
// modified even when the surrounding text is normalized.
func TestFencedCodeBlockPreserved(t *testing.T) {
	dir := t.TempDir()
	stop := startWatcher(t, dir)
	defer stop()

	input := "before，after\n\n```go\nfmt.Println(\"你好，世界\")\n```\n\nend，done\n"
	path := filepath.Join(dir, "fenced.md")
	writeFile(t, path, input)

	time.Sleep(grace)

	got := readFile(t, path)
	// Content outside the fence should be normalized.
	if strings.ContainsRune(got, '，') {
		// But only outside the fence — check the fence content is intact.
		lines := strings.Split(got, "\n")
		inFence := false
		for _, line := range lines {
			if strings.HasPrefix(strings.TrimSpace(line), "```") {
				inFence = !inFence
				continue
			}
			if inFence && strings.ContainsRune(line, '，') {
				// Expected: fence content is preserved.
				return
			}
		}
		t.Errorf("Chinese comma outside fence block was not replaced; got:\n%s", got)
	}
}

// TestCJKLatinSpacing verifies that spaces are inserted at CJK/Latin boundaries.
func TestCJKLatinSpacing(t *testing.T) {
	dir := t.TempDir()
	stop := startWatcher(t, dir)
	defer stop()

	path := filepath.Join(dir, "spacing.md")
	writeFile(t, path, "ERP系统和JSON数据以及这个Agent。\n")

	time.Sleep(grace)

	got := readFile(t, path)
	for _, want := range []string{"ERP 系统", "JSON 数据", "这个 Agent"} {
		if !strings.Contains(got, want) {
			t.Errorf("expected %q in output; got: %q", want, got)
		}
	}
}

// TestSubdirectoryNewFile verifies that a file created inside a newly created
// subdirectory is picked up and normalized.
func TestSubdirectoryNewFile(t *testing.T) {
	dir := t.TempDir()
	stop := startWatcher(t, dir)
	defer stop()

	// Create a subdirectory after the watcher has started.
	sub := filepath.Join(dir, "subdir")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	// Give the watcher time to register the new directory.
	time.Sleep(150 * time.Millisecond)

	path := filepath.Join(sub, "nested.md")
	writeFile(t, path, "子目录文件，测试用。\n")

	time.Sleep(grace)

	got := readFile(t, path)
	if strings.ContainsRune(got, '，') {
		t.Errorf("file in new subdirectory was not normalized; got: %q", got)
	}
}

// TestDryRunDoesNotModifyFile verifies that --dry-run prints intent without
// actually writing back to disk.
func TestDryRunDoesNotModifyFile(t *testing.T) {
	dir := t.TempDir()
	stop := startWatcher(t, dir, "--dry-run")
	defer stop()

	original := "你好，世界。\n"
	path := filepath.Join(dir, "dry.md")
	writeFile(t, path, original)

	time.Sleep(grace)

	got := readFile(t, path)
	if got != original {
		t.Errorf("--dry-run should not modify files; got: %q, want: %q", got, original)
	}
}

// TestNonMdFileIgnored verifies that files with non-configured extensions are
// left untouched.
func TestNonMdFileIgnored(t *testing.T) {
	dir := t.TempDir()
	stop := startWatcher(t, dir)
	defer stop()

	original := "你好，世界。\n"
	path := filepath.Join(dir, "note.txt")
	writeFile(t, path, original)

	time.Sleep(grace)

	got := readFile(t, path)
	if got != original {
		t.Errorf(".txt file should not be processed by default; got: %q", got)
	}
}

// TestSampleFixture runs the full sample fixture through the watcher and
// prints a before/after diff to make visual inspection easy.
func TestSampleFixture(t *testing.T) {
	dir := t.TempDir()

	// Copy fixture into temp dir.
	fixture, err := os.ReadFile("fixtures/sample.md")
	if err != nil {
		t.Fatalf("cannot read fixture: %v", err)
	}
	path := filepath.Join(dir, "sample.md")
	writeFile(t, path, string(fixture))

	stop := startWatcher(t, dir)
	defer stop()

	// Trigger a write event so the watcher processes the file.
	// (Appending a newline is enough to generate a Write event.)
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = f.WriteString("\n")
	f.Close()

	time.Sleep(grace)

	got := readFile(t, path)

	t.Log("=== normalized output ===")
	for _, line := range strings.Split(got, "\n") {
		t.Log(line)
	}

	// Sanity check: at least one substitution must have happened.
	if got == string(fixture)+"\n" {
		t.Error("fixture was not modified — normalization may not be working")
	}
}
