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

	"punctpolish/internal/app"
	"punctpolish/internal/config"
	"punctpolish/internal/logging"
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
		logFile     = fs.String("log-file", "", "path to log file (default: $XDG_STATE_HOME/punctpolish/punctpolish.log)")
	)

	if err := fs.Parse(os.Args[1:]); err != nil {
		resolvedLog, closeLog, logErr := logging.Setup("", "", false)
		if logErr == nil {
			defer closeLog()
			slog.Error("invalid command line arguments", "error", err)
		}
		printStartupError(err.Error(), resolvedLog)
		os.Exit(2)
	}

	resolvedLog, closeLog, err := logging.Setup(*logFile, *logLevel, *foreground)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: cannot initialize logger: %v\n", err)
		os.Exit(1)
	}
	defer closeLog()

	if runtime.GOOS != "darwin" {
		fatal(resolvedLog, "punctpolish only supports macOS")
	}

	if *dir == "" {
		fatal(resolvedLog, "missing required flag: --dir")
	}

	absDir, err := resolveDir(*dir)
	if err != nil {
		fatal(resolvedLog, fmt.Sprintf("cannot access --dir %q: %v", *dir, err))
	}

	cfg, err := config.Load(*cfgFile, absDir)
	if err != nil {
		fatal(resolvedLog, fmt.Sprintf("cannot load config: %v", err))
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
		fatal(resolvedLog, fmt.Sprintf("fatal error: %v", err))
	}
}

// resolveDir validates that path exists and is a directory, then returns its
// absolute path.
func resolveDir(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(abs)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", fmt.Errorf("%q is not a directory", abs)
	}
	return abs, nil
}

// fatal logs msg at error level, prints a startup error to stderr, and exits.
func fatal(logFile, msg string) {
	slog.Error(msg, "log_file", logFile)
	printStartupError(msg, logFile)
	os.Exit(1)
}

func printStartupError(message, logFile string) {
	name := filepath.Base(os.Args[0])
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
	fmt.Fprintf(&b, "  %s --dir /path/to/docs\n", name)
	fmt.Fprintf(&b, "  %s --dir /path/to/docs --log-level debug\n", name)
	fmt.Fprintf(&b, "  %s --dir /path/to/docs --foreground --log-level debug\n", name)
	fmt.Fprint(os.Stderr, b.String())
}
