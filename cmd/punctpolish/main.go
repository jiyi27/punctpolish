package main

import (
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"punctpolish/internal/app"
	"punctpolish/internal/config"
)

func main() {
	fs := flag.NewFlagSet(os.Args[0], flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	var (
		dir         = fs.String("dir", "", "root directory to watch (required)")
		cfgFile     = fs.String("config", "", "path to config file (default: auto-discover .punctpolish.yaml)")
		scanOnStart = fs.Bool("scan-on-start", false, "process all matching files once before entering watch mode")
		dryRun      = fs.Bool("dry-run", false, "print what would change without writing files")
		foreground  = fs.Bool("foreground", false, "also write runtime logs to stderr")
		debounce    = fs.Duration("debounce", 0, "debounce duration (e.g. 300ms); overrides config file")
		logLevel    = fs.String("log-level", "", "log verbosity: debug|info|warn|error (default: warn)")
	)

	if err := fs.Parse(os.Args[1:]); err != nil {
		logFile, closeLog, logErr := setupLogger("", false)
		if logErr == nil {
			defer closeLog()
			slog.Error("invalid command line arguments", "error", err, "log_file", logFile)
		}
		printStartupError(err.Error(), logFile)
		os.Exit(2)
	}

	logFile, closeLog, err := setupLogger(*logLevel, *foreground)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: cannot initialize logger: %v\n", err)
		os.Exit(1)
	}
	defer closeLog()

	// Verify that we are running on macOS before doing anything else.
	if runtime.GOOS != "darwin" {
		slog.Error("punctpolish only supports macOS", "os", runtime.GOOS, "log_file", logFile)
		printStartupError("punctpolish only supports macOS", logFile)
		os.Exit(1)
	}

	if *dir == "" {
		slog.Error("missing required flag", "flag", "--dir", "log_file", logFile)
		printStartupError("missing required flag: --dir", logFile)
		os.Exit(1)
	}

	// Resolve the absolute path so log messages are unambiguous.
	absDir, err := resolveDir(*dir)
	if err != nil {
		slog.Error("cannot access watch directory", "dir", *dir, "error", err, "log_file", logFile)
		printStartupError(fmt.Sprintf("cannot access --dir %q: %v", *dir, err), logFile)
		os.Exit(1)
	}

	// Load config file, then apply CLI overrides.
	cfg, err := config.Load(*cfgFile, absDir)
	if err != nil {
		slog.Error("cannot load config", "config", *cfgFile, "error", err, "log_file", logFile)
		printStartupError(fmt.Sprintf("cannot load config: %v", err), logFile)
		os.Exit(1)
	}

	cfg.ScanOnStart = *scanOnStart
	cfg.DryRun = *dryRun

	if *debounce > 0 {
		cfg.Debounce = *debounce
	}
	if *logLevel != "" {
		cfg.LogLevel = *logLevel
	}

	if err := app.New(cfg, absDir).Run(); err != nil {
		slog.Error("fatal error", "error", err, "log_file", logFile)
		printStartupError(fmt.Sprintf("fatal error: %v", err), logFile)
		os.Exit(1)
	}
}

// resolveDir validates that path exists and is a directory, then returns its
// cleaned absolute path.
func resolveDir(path string) (string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", fmt.Errorf("%q is not a directory", path)
	}
	abs, err := resolveAbs(path)
	if err != nil {
		return "", err
	}
	return abs, nil
}

// resolveAbs returns the absolute path of p using os.Getwd as the base.
func resolveAbs(p string) (string, error) {
	if len(p) > 0 && p[0] == '/' {
		return p, nil
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	return cwd + "/" + p, nil
}

// setupLogger initialises the global slog logger with the requested level and
// writes logs to punctpolish.log in the current working directory.
func setupLogger(level string, foreground bool) (string, func(), error) {
	logPath, err := defaultLogPath()
	if err != nil {
		return "", func() {}, err
	}

	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
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
			// Format timestamps as HH:MM:SS for readability.
			if a.Key == slog.TimeKey {
				a.Value = slog.StringValue(a.Value.Time().Format(time.RFC3339))
			}
			return a
		},
	})
	slog.SetDefault(slog.New(handler))
	return logPath, func() { _ = f.Close() }, nil
}

func defaultLogPath() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	return filepath.Join(cwd, "punctpolish.log"), nil
}

func printStartupError(message, logFile string) {
	var b strings.Builder
	b.WriteString("error: ")
	b.WriteString(message)
	b.WriteString("\n")
	if logFile != "" {
		b.WriteString("log file: ")
		b.WriteString(logFile)
		b.WriteString("\n")
	}
	b.WriteString("example:\n")
	b.WriteString("  punctpolish --dir /path/to/docs\n")
	b.WriteString("  punctpolish --dir /path/to/docs --log-level debug\n")
	b.WriteString("  punctpolish --dir /path/to/docs --foreground --log-level debug\n")
	fmt.Fprint(os.Stderr, b.String())
}
