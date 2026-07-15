package test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"punctpolish/internal/processor"
)

func buildBinary(t *testing.T) string {
	t.Helper()

	binary := filepath.Join(t.TempDir(), "punctpolish")
	cmd := exec.Command("go", "build", "-o", binary, "../cmd/punctpolish")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build punctpolish: %v\n%s", err, output)
	}
	return binary
}

func run(t *testing.T, binary string, args ...string) {
	t.Helper()

	cmd := exec.Command(binary, args...)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("run punctpolish: %v\n%s", err, output)
	}
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
		t.Fatal(err)
	}
	return string(b)
}

func TestDirectoryProcessesOnlyMarkdownAndTextFiles(t *testing.T) {
	root := t.TempDir()
	markdown := filepath.Join(root, "nested", "note.MD")
	text := filepath.Join(root, "plain.txt")
	other := filepath.Join(root, "source.go")
	markdownOriginal := "结果：成功！\n"
	textOriginal := "ERP系统和JSON数据。\n"
	otherOriginal := "结果：成功！\n"
	writeFile(t, markdown, markdownOriginal)
	writeFile(t, text, textOriginal)
	writeFile(t, other, otherOriginal)

	run(t, buildBinary(t), root)

	if got, want := readFile(t, markdown), processor.NormalizeText(markdownOriginal); got != want {
		t.Errorf("markdown = %q, want %q", got, want)
	}
	if got, want := readFile(t, text), processor.NormalizeText(textOriginal); got != want {
		t.Errorf("text = %q, want %q", got, want)
	}
	if got := readFile(t, other); got != otherOriginal {
		t.Errorf("non-allowlisted file changed: %q", got)
	}
}

func TestSingleFileAcceptsAnyTextExtensionAndSkipsBinaryContent(t *testing.T) {
	root := t.TempDir()
	logFile := filepath.Join(root, "note.log")
	binaryFile := filepath.Join(root, "image.bin")
	original := "结果：成功！\n"
	writeFile(t, logFile, original)
	if err := os.WriteFile(binaryFile, []byte{0, 1, 2, 3}, 0o644); err != nil {
		t.Fatal(err)
	}

	binary := buildBinary(t)
	run(t, binary, logFile)
	run(t, binary, binaryFile)

	if got, want := readFile(t, logFile), processor.NormalizeText(original); got != want {
		t.Errorf("text file = %q, want %q", got, want)
	}
	if got, want := readFile(t, binaryFile), string([]byte{0, 1, 2, 3}); got != want {
		t.Errorf("binary file changed: %q", got)
	}
}
