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

func TestNormalizeText_DoesNotInsertSpacesInsideMarkdownImagePath(t *testing.T) {
	input := "见下图：\n![n8n_node_explain_002](./006-运行n8n.assets/n8n_node_explain_002.png)\n"

	got := NormalizeText(input)
	want := "见下图: \n![n8n_node_explain_002](./006-运行n8n.assets/n8n_node_explain_002.png)\n"
	if got != want {
		t.Fatalf("NormalizeText() should preserve markdown image paths\ngot:  %q\nwant: %q", got, want)
	}
}

func TestNormalizeText_DoesNotInsertSpacesInsideBareURL(t *testing.T) {
	input := "参考https://example.com/运行n8n/流程 和Agent说明\n"

	got := NormalizeText(input)
	want := "参考 https://example.com/运行n8n/流程 和 Agent 说明\n"
	if got != want {
		t.Fatalf("NormalizeText() should preserve bare URLs while still spacing surrounding text\ngot:  %q\nwant: %q", got, want)
	}
}
