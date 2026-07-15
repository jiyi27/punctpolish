# punctpolish

`punctpolish` 批量整理中文与中英文混排文本：将常见中文标点替换为 ASCII 标点，并补齐中文与英文、数字之间的空格。Markdown fenced code block 会保持原样。

## 使用

```bash
punctpolish <文件或目录>
```

指定文件时，工具会处理任意扩展名，但仅在内容被识别为文本时才会写回；二进制文件会跳过。

```bash
punctpolish ./note.log
```

指定目录时，工具会递归处理目录及其子目录中的 `.md` 和 `.txt` 文件（不区分扩展名大小写）。其他格式不会处理。

```bash
punctpolish ./docs
```

## 构建与测试

```bash
go build -o punctpolish ./cmd/punctpolish
go test ./...
```
