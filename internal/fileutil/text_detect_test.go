package fileutil

import (
	"os"
	"testing"
)

func writeTempFile(t *testing.T, content []byte) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "detect-*")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.Write(content); err != nil {
		t.Fatal(err)
	}
	f.Close()
	return f.Name()
}

func TestIsTextFile_PlainUTF8(t *testing.T) {
	path := writeTempFile(t, []byte("Hello, 世界\n这是一个测试文件。\n"))
	ok, err := IsTextFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Error("expected text file, got binary")
	}
}

func TestIsTextFile_WithBOM(t *testing.T) {
	bom := []byte{0xEF, 0xBB, 0xBF}
	path := writeTempFile(t, append(bom, []byte("Hello world")...))
	ok, err := IsTextFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Error("expected BOM file to be detected as text")
	}
}

func TestIsTextFile_Binary(t *testing.T) {
	// Simulate a binary file with null bytes.
	data := make([]byte, 512)
	for i := range data {
		data[i] = byte(i % 256)
	}
	path := writeTempFile(t, data)
	ok, err := IsTextFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Error("expected binary file to be detected as non-text")
	}
}

func TestIsTextFile_EmptyFile(t *testing.T) {
	path := writeTempFile(t, []byte{})
	// An empty file read returns 0 bytes and likely EOF; should not crash.
	_, _ = IsTextFile(path)
}
