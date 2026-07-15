# punctpolish

## Commands

```bash
go build -o punctpolish ./cmd/punctpolish
go test ./...
```

## Architecture

`punctpolish <file-or-directory>` is a one-shot text normalizer.

- A file target is processed when its contents look like text, regardless of extension.
- A directory target is walked recursively and only `.md` and `.txt` files are processed.
- `internal/processor` owns text detection, size limits, normalization, and safe write-back.
- `internal/scanner` owns the fixed directory extension allowlist.
