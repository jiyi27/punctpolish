package app

import (
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"punctpolish/internal/config"
	"punctpolish/internal/fileutil"
	"punctpolish/internal/processor"
	"punctpolish/internal/watcher"
)

// App wires all components together and manages the process lifecycle.
type App struct {
	cfg     config.Config
	rootDir string
}

// New creates an App with the given configuration and root directory.
func New(cfg config.Config, rootDir string) *App {
	return &App{cfg: cfg, rootDir: rootDir}
}

// Run starts all components and blocks until SIGINT or SIGTERM is received.
func (a *App) Run() error {
	guard := fileutil.NewWriteGuard(config.DefaultWriteGap)
	proc := processor.New(guard, a.cfg.MaxFileSize, a.cfg.DryRun)

	w, err := watcher.New(a.cfg, proc, guard)
	if err != nil {
		return err
	}
	defer func() {
		if err := w.Close(); err != nil {
			slog.Warn("error closing watcher", "error", err)
		}
	}()

	// Register the root directory and all its subdirectories.
	if err := w.AddDir(a.rootDir); err != nil {
		return err
	}

	// Optionally process all existing files before entering watch mode.
	if a.cfg.ScanOnStart {
		slog.Info("scan-on-start: processing existing files", "dir", a.rootDir)
		w.ScanAndProcess(a.rootDir)
		slog.Info("scan-on-start: done, entering watch mode")
	}

	slog.Info("watching for changes", "dir", a.rootDir)

	done := make(chan struct{})
	go w.Run(done)

	// Wait for termination signal.
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("shutting down")
	close(done)
	return nil
}
