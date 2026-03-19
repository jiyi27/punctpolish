# punctpolish

`punctpolish` is a small macOS file watcher that automatically normalizes text files when they change.

It is designed for mixed Chinese and English writing workflows, especially Markdown documents that need cleaner punctuation and spacing without touching fenced code blocks.

## Features

- Recursively watches a directory and its subdirectories
- Processes matching files on create, write, and rename events
- Supports configurable file extensions, ignored directories, debounce delay, and file size limits
- Rewrites Chinese punctuation into ASCII punctuation outside fenced code blocks
- Inserts spacing between CJK and Latin/digit text such as `ERP 系统` and `JSON 数据`
- Normalizes comma spacing and collapses repeated spaces
- Removes trailing punctuation at paragraph endings and list items
- Skips binary files and suppresses self-triggered write loops
- Supports `--dry-run` and `--scan-on-start`

## Requirements

- macOS only
- Go 1.23.12 or newer

The binary exits immediately on non-macOS systems.

## Installation

Build from source:

```bash
go build -o punctpolish ./cmd/punctpolish
```

Or run directly with Go:

```bash
go run ./cmd/punctpolish --dir /path/to/docs
```

## Usage

Basic usage:

```bash
./punctpolish --dir /path/to/docs
```

Common options:

```bash
./punctpolish \
  --dir /path/to/docs \
  --scan-on-start \
  --debounce 300ms \
  --log-level debug
```

### CLI flags

- `--dir`: root directory to watch, required
- `--config`: explicit config file path
- `--scan-on-start`: process existing matching files before entering watch mode
- `--dry-run`: log what would change without writing files
- `--debounce`: override debounce duration, for example `300ms`
- `--log-level`: `debug`, `info`, `warn`, or `error`

## Configuration

If `--config` is not provided, `punctpolish` looks for `.punctpolish.yaml` in this order:

1. Current working directory
2. The watched directory passed to `--dir`
3. `$HOME`

Example:

```yaml
ext:
  - .md
  - .txt

ignore:
  - .git
  - node_modules
  - dist
  - build

debounce: 500ms
max_file_size: 10485760
```

### Config fields

- `ext`: file extensions to process
- `ignore`: directory names to skip entirely
- `debounce`: debounce delay before processing a changed file
- `max_file_size`: maximum file size in bytes

Default behavior:

- `ext`: `[".md"]`
- `ignore`: `[".git", "node_modules", ".idea", ".vscode", "dist", "build"]`
- `debounce`: `500ms`
- `max_file_size`: `10485760` bytes (10 MB)

## What Gets Normalized

For supported text files, `punctpolish` currently applies these transformations outside fenced code blocks:

- Chinese punctuation to ASCII equivalents, for example `，` to `, ` and `！` to `! `
- Chinese brackets and quotes to ASCII forms
- Spacing between CJK and Latin/digit boundaries
- Comma spacing normalization
- Repeated space collapsing inside a line
- Trailing punctuation removal at paragraph endings and list items

Example:

```text
ERP系统已经上线，运行稳定！
这个Agent负责处理JSON数据。
```

Becomes:

```text
ERP 系统已经上线, 运行稳定
这个 Agent 负责处理 JSON 数据
```

Fenced blocks such as triple-backtick code blocks are preserved as-is.

## Testing

Run unit tests:

```bash
go test ./...
```

Run integration tests:

```bash
go test -tags integration -v ./test/
```

There is also a manual smoke test script:

```bash
./test/run.sh
```

## Project Structure

- `cmd/punctpolish`: CLI entrypoint
- `internal/app`: application wiring and lifecycle
- `internal/config`: config loading and defaults
- `internal/watcher`: recursive fsnotify watcher
- `internal/processor`: text normalization pipeline
- `internal/fileutil`: text detection and write-loop protection
- `test`: integration tests and smoke test script

## Notes

- The watcher only processes files whose extension matches the configured list.
- New subdirectories created after startup are added to the watch set automatically.
- Binary files are skipped even if their extension matches.
