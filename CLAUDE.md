# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

```bash
# Build
go build -o punctpolish ./cmd/punctpolish

# Run all unit tests
go test ./...

# Run integration tests (require building binary and real FS operations)
go test -tags integration -v ./test

# Run a specific unit test
go test -run TestNormalizeText_ReplacesCorePunctuation ./internal/processor

# Run the example preview test
go test -run ExampleNormalizeText_preview -v ./internal/processor

# Manual smoke test
bash test/run.sh [--keep]
```

## Architecture

**punctpolish** watches directories (or processes individual files) and automatically normalizes Chinese/mixed-language text: replacing Chinese punctuation with ASCII equivalents, and inserting spaces at CJK↔Latin/digit boundaries.

### Three operating modes (mutually exclusive CLI flags)
- `--dir <path>` — watch mode: continuously monitors for changes via fsnotify
- `--file <path>` — single-file mode: processes once and exits
- `--scan <path>` — batch mode: recursively processes all matching files and exits

### Core pipeline: `internal/processor/normalize.go`
Text normalization runs line-by-line through 5 stages:
1. Replace Chinese punctuation (，。！？ → ,.!?)
2. Normalize comma spacing
3. Insert spaces at CJK↔Latin/digit boundaries (Markdown-aware; preserves URLs and link syntax)
4. Collapse consecutive spaces (preserving leading whitespace)
5. Strip trailing punctuation at paragraph ends

### Key components
- **`cmd/punctpolish/main.go`** — CLI parsing, mode validation, routing
- **`internal/app/app.go`** — wires components, signal handling (SIGINT/SIGTERM), optional `--scan-on-start`
- **`internal/config/config.go`** — YAML config loading with 4-step search (explicit → cwd → target dir → $HOME); defaults: `.md` ext, 500ms debounce, 10 MB max size
- **`internal/watcher/watcher.go`** — fsnotify wrapper with per-file debounce timers, extension/ignore filtering, auto-watch new subdirs
- **`internal/scanner/scanner.go`** — shared `Walk()` used by both watcher and scan modes; `Filter` interface for extension/dir matching
- **`internal/fileutil/write_guard.go`** — tracks recently-written files to suppress the fsnotify events that the watcher itself triggers (prevents infinite loops)
- **`internal/fileutil/text_detect.go`** — binary content detection (reads first 8KB, checks for null bytes)
- **`internal/logging/logging.go`** — slog-based logging to `~/.local/state/punctpolish/punctpolish.log`; `--foreground` adds stderr output

### Avoiding infinite loops
WriteGuard (`internal/fileutil/write_guard.go`) records each file write with a timestamp. The watcher skips events for files within the guard window (default 1s), preventing the watcher from re-processing files it just wrote.

### Integration tests
Tests in `test/integration_test.go` use `//go:build integration` and spawn the actual binary as a subprocess. They validate:
- Only changed files are processed (not untouched files)
- Pre-existing files are not touched on startup without `--scan-on-start`
- `--scan-on-start` processes all pre-existing matching files

## Coding Conventions

- **Code Comments**: Prefer purpose-driven comments that explain intent or constraints, not comments that just restate variable names or obvious code behavior.