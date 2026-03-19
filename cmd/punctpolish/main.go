package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"
	"runtime"
	"time"

	"punctpolish/internal/app"
	"punctpolish/internal/config"
)

func main() {
	// Verify that we are running on macOS before doing anything else.
	if runtime.GOOS != "darwin" {
		fmt.Fprintln(os.Stderr, "error: punctpolish only supports macOS (darwin)")
		os.Exit(1)
	}

	var (
		dir         = flag.String("dir", "", "root directory to watch (required)")
		cfgFile     = flag.String("config", "", "path to config file (default: auto-discover .punctpolish.yaml)")
		scanOnStart = flag.Bool("scan-on-start", false, "process all matching files once before entering watch mode")
		dryRun      = flag.Bool("dry-run", false, "print what would change without writing files")
		debounce    = flag.Duration("debounce", 0, "debounce duration (e.g. 300ms); overrides config file")
		logLevel    = flag.String("log-level", "", "log verbosity: debug|info|warn|error (default: info)")
	)
	flag.Parse()

	if *dir == "" {
		fmt.Fprintln(os.Stderr, "error: --dir is required")
		flag.Usage()
		os.Exit(1)
	}

	// Resolve the absolute path so log messages are unambiguous.
	absDir, err := resolveDir(*dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: cannot access --dir %q: %v\n", *dir, err)
		os.Exit(1)
	}

	// Load config file, then apply CLI overrides.
	cfg, err := config.Load(*cfgFile, absDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: cannot load config: %v\n", err)
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

	setupLogger(cfg.LogLevel)

	if err := app.New(cfg, absDir).Run(); err != nil {
		slog.Error("fatal error", "error", err)
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
// a human-readable text handler.
func setupLogger(level string) {
	var lvl slog.Level
	switch level {
	case "debug":
		lvl = slog.LevelDebug
	case "warn":
		lvl = slog.LevelWarn
	case "error":
		lvl = slog.LevelError
	default:
		lvl = slog.LevelInfo
	}

	handler := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: lvl,
		ReplaceAttr: func(_ []string, a slog.Attr) slog.Attr {
			// Format timestamps as HH:MM:SS for readability.
			if a.Key == slog.TimeKey {
				a.Value = slog.StringValue(a.Value.Time().Format(time.TimeOnly))
			}
			return a
		},
	})
	slog.SetDefault(slog.New(handler))
}
