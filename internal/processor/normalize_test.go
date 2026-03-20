package processor

import "testing"

func TestNormalizeText_ReplacesCorePunctuationAndSpacing(t *testing.T) {
	input := "ERP系统和JSON数据：请联系admin@example.com！\n"

	got := NormalizeText(input)
	want := "ERP 系统和 JSON 数据: 请联系 admin@example.com\n"
	if got != want {
		t.Fatalf("NormalizeText() mismatch\ngot:  %q\nwant: %q", got, want)
	}
}

func TestNormalizeText_AlsoProcessesFencedCodeBlocks(t *testing.T) {
	input := "外部，内容。\n\n```go\nfmt.Println(\"你好，世界！\")\n```\n"

	got := NormalizeText(input)
	want := "外部, 内容\n\n```go\nfmt.Println(\"你好, 世界! \")\n```\n"
	if got != want {
		t.Fatalf("NormalizeText() should process fenced code blocks too\ngot:  %q\nwant: %q", got, want)
	}
}
