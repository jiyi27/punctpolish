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

You can also place the binary somewhere stable, for example:

```bash
mkdir -p ./bin
go build -o ./bin/punctpolish ./cmd/punctpolish
```

Or run directly with Go:

```bash
go run ./cmd/punctpolish --dir /path/to/docs
```

## Usage

After building, start the watcher by passing the directory to watch:

```bash
./punctpolish --dir /path/to/docs
```

If you built into `./bin`, run:

```bash
./bin/punctpolish --dir /path/to/docs
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

The config file name is `.punctpolish.yaml`.

If `--config` is not provided, `punctpolish` looks for `.punctpolish.yaml` in this order and uses the first file it finds:

1. Current working directory
2. The watched directory passed to `--dir`
3. `$HOME`

If no config file is found, the program runs with built-in defaults.

Example:

```yaml
ext:
  - .md
  - .txt

ignore:
  - .git
  - node_modules
  - .idea
  - .vscode
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

### Config vs CLI flags

The effective runtime config is resolved in this order:

1. Built-in defaults
2. Values from `.punctpolish.yaml`
3. CLI flags

That means CLI flags have the highest priority.

Examples:

- `--debounce 300ms` overrides `debounce: 500ms` from the config file
- `--config /path/to/custom.yaml` changes which config file is loaded

Current support is split like this:

- Config file only: `ext`, `ignore`, `max_file_size`
- Config file or CLI: `debounce`
- CLI only: `--dir`, `--config`, `--scan-on-start`, `--dry-run`, `--log-level`

### Recommended setup

For a typical docs directory, create a `.punctpolish.yaml` like this:

```yaml
ext:
  - .md
  - .txt

ignore:
  - .git
  - node_modules
  - .idea
  - .vscode
  - dist
  - build

debounce: 500ms
max_file_size: 10485760
```

Then run:

```bash
./punctpolish --dir /path/to/docs
```

Or, if you want to preview changes without rewriting files:

```bash
./punctpolish --dir /path/to/docs --dry-run --scan-on-start
```

## Run As A Background Service

On macOS, the usual way to keep `punctpolish` running is `launchd`.

### 1. Build the binary to a fixed path

Example:

```bash
mkdir -p "$HOME/.local/bin"
go build -o "$HOME/.local/bin/punctpolish" ./cmd/punctpolish
```

### 2. Create a `launchd` plist

Create `~/Library/LaunchAgents/com.example.punctpolish.plist`:

```xml
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>Label</key>
  <string>com.example.punctpolish</string>

  <key>ProgramArguments</key>
  <array>
    <string>/Users/yourname/.local/bin/punctpolish</string>
    <string>--dir</string>
    <string>/Users/yourname/path/to/docs</string>
    <string>--config</string>
    <string>/Users/yourname/path/to/.punctpolish.yaml</string>
    <string>--scan-on-start</string>
  </array>

  <key>RunAtLoad</key>
  <true/>

  <key>KeepAlive</key>
  <true/>

  <key>WorkingDirectory</key>
  <string>/Users/yourname/path/to/docs</string>

  <key>StandardOutPath</key>
  <string>/Users/yourname/Library/Logs/punctpolish.log</string>

  <key>StandardErrorPath</key>
  <string>/Users/yourname/Library/Logs/punctpolish.error.log</string>
</dict>
</plist>
```

Replace the sample paths with your real absolute paths.

### 3. Load and manage the service

Load it:

```bash
launchctl load ~/Library/LaunchAgents/com.example.punctpolish.plist
```

Start immediately:

```bash
launchctl start com.example.punctpolish
```

Stop it:

```bash
launchctl stop com.example.punctpolish
```

Unload it:

```bash
launchctl unload ~/Library/LaunchAgents/com.example.punctpolish.plist
```

### 4. Verify it is running

```bash
launchctl list | grep punctpolish
```

If startup fails, inspect:

- `~/Library/Logs/punctpolish.log`
- `~/Library/Logs/punctpolish.error.log`

### Notes for background use

- Use absolute paths in the plist for the binary, watched directory, and config file.
- Keep the binary in a stable location so future rebuilds do not break the service.
- If you update the binary or plist, unload and load the agent again.
- `WorkingDirectory` helps config auto-discovery, but using `--config` is the most explicit and reliable setup for a background service.

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
