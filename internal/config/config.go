package config

import (
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

const (
	configFileName  = ".textwatcher.yaml"
	DefaultDebounce = 500 * time.Millisecond
	DefaultWriteGap = 1 * time.Second
	DefaultMaxBytes = 10 * 1024 * 1024 // 10 MB
)

// Config holds all runtime configuration.
type Config struct {
	// Extensions is the list of file extensions to watch (e.g. [".md", ".txt"]).
	Extensions []string `yaml:"ext"`

	// IgnoreDirs is the list of directory names to skip entirely.
	IgnoreDirs []string `yaml:"ignore"`

	// Debounce is how long to wait after the last event before processing a file.
	Debounce time.Duration `yaml:"debounce"`

	// MaxFileSize is the maximum file size in bytes that will be processed.
	MaxFileSize int64 `yaml:"max_file_size"`

	// DryRun prints what would change without writing back to disk.
	DryRun bool `yaml:"-"`

	// ScanOnStart processes all matching files once before entering watch mode.
	ScanOnStart bool `yaml:"-"`

	// LogLevel controls log verbosity: debug, info, warn, error.
	LogLevel string `yaml:"-"`
}

// Default returns a Config populated with sensible defaults.
func Default() Config {
	return Config{
		Extensions:  []string{".md"},
		IgnoreDirs:  []string{".git", "node_modules", ".idea", ".vscode", "dist", "build"},
		Debounce:    DefaultDebounce,
		MaxFileSize: DefaultMaxBytes,
		LogLevel:    "info",
	}
}

// Load reads a YAML config file and merges it onto a default Config.
// It searches in the following order and uses the first file found:
//  1. path (explicit --config flag, may be empty)
//  2. current working directory
//  3. watchDir (the --dir argument)
//  4. $HOME
func Load(path, watchDir string) (Config, error) {
	cfg := Default()

	candidate := resolve(path, watchDir)
	if candidate == "" {
		// No config file found; return defaults.
		return cfg, nil
	}

	data, err := os.ReadFile(candidate)
	if err != nil {
		return cfg, err
	}

	// file only needs to specify fields it wants to override
	var file fileConfig
	if err := yaml.Unmarshal(data, &file); err != nil {
		return cfg, err
	}

	if len(file.Extensions) > 0 {
		cfg.Extensions = file.Extensions
	}
	if len(file.IgnoreDirs) > 0 {
		cfg.IgnoreDirs = file.IgnoreDirs
	}
	if file.Debounce > 0 {
		cfg.Debounce = file.Debounce
	}
	if file.MaxFileSize > 0 {
		cfg.MaxFileSize = file.MaxFileSize
	}

	return cfg, nil
}

// fileConfig is a YAML-only struct so that time.Duration parses correctly.
type fileConfig struct {
	Extensions  []string      `yaml:"ext"`
	IgnoreDirs  []string      `yaml:"ignore"`
	Debounce    time.Duration `yaml:"debounce"`
	MaxFileSize int64         `yaml:"max_file_size"`
}

// resolve finds the first readable config file among the candidate paths.
func resolve(explicit, watchDir string) string {
	var candidates []string

	if explicit != "" {
		candidates = append(candidates, explicit)
	}

	if cwd, err := os.Getwd(); err == nil {
		candidates = append(candidates, filepath.Join(cwd, configFileName))
	}

	if watchDir != "" {
		candidates = append(candidates, filepath.Join(watchDir, configFileName))
	}

	if home, err := os.UserHomeDir(); err == nil {
		candidates = append(candidates, filepath.Join(home, configFileName))
	}

	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}
