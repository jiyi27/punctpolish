package logging

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"time"
)

// Setup initializes the global slog logger and opens the log file.
//
// logFile is the explicit path (from --log-file flag); if empty, the path
// defaults to $XDG_STATE_HOME/punctpolish/punctpolish.log, falling back to
// $HOME/.local/state/punctpolish/punctpolish.log.
//
// Returns the resolved log file path, a close function, and any error.
func Setup(logFile, level string, foreground bool) (string, func(), error) {
	path, err := resolvePath(logFile)
	if err != nil {
		return "", func() {}, err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", func() {}, err
	}

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return "", func() {}, err
	}

	var lvl slog.Level
	switch level {
	case "debug":
		lvl = slog.LevelDebug
	case "error":
		lvl = slog.LevelError
	case "info":
		lvl = slog.LevelInfo
	default:
		lvl = slog.LevelWarn
	}

	writer := io.Writer(f)
	if foreground {
		writer = io.MultiWriter(f, os.Stderr)
	}

	handler := slog.NewTextHandler(writer, &slog.HandlerOptions{
		Level: lvl,
		ReplaceAttr: func(_ []string, a slog.Attr) slog.Attr {
			if a.Key == slog.TimeKey {
				a.Value = slog.StringValue(a.Value.Time().Format(time.RFC3339))
			}
			return a
		},
	})
	slog.SetDefault(slog.New(handler))
	return path, func() { _ = f.Close() }, nil
}

// resolvePath returns the log file path to use.
// explicit takes priority; otherwise $XDG_STATE_HOME or ~/.local/state is used.
func resolvePath(explicit string) (string, error) {
	if explicit != "" {
		return explicit, nil
	}

	stateDir := os.Getenv("XDG_STATE_HOME")
	if stateDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		stateDir = filepath.Join(home, ".local", "state")
	}

	return filepath.Join(stateDir, "punctpolish", "punctpolish.log"), nil
}
