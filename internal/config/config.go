// Package config provides configuration management for HAL 9000.
// Configuration is loaded from ~/.config/hal9000/config.yaml with sensible defaults.
// Project-relative paths (library, services) are resolved from the executable's directory.
package config

import (
	"os"
	"path/filepath"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"
)

var (
	// executableDir caches the executable's directory
	executableDir     string
	executableDirOnce sync.Once
)

// Config holds the HAL 9000 configuration.
type Config struct {
	Library LibraryConfig `yaml:"library"`
}

// LibraryConfig holds library-related configuration.
type LibraryConfig struct {
	Path string `yaml:"path"`
}

var (
	globalConfig *Config
	configOnce   sync.Once
	configErr    error
)

const (
	// DefaultConfigPath is the default location for the config file.
	DefaultConfigPath = "~/.config/hal9000/config.yaml"

	// DefaultLibraryPath is the default library path when no config is present.
	DefaultLibraryPath = "./library"
)

// Load loads the configuration from the default path.
// It returns the cached config on subsequent calls.
func Load() (*Config, error) {
	configOnce.Do(func() {
		globalConfig, configErr = loadFromPath(DefaultConfigPath)
	})
	return globalConfig, configErr
}

// loadFromPath loads configuration from a specific file path.
func loadFromPath(path string) (*Config, error) {
	cfg := &Config{
		Library: LibraryConfig{
			Path: DefaultLibraryPath,
		},
	}

	expandedPath := expandPath(path)
	data, err := os.ReadFile(expandedPath)
	if err != nil {
		if os.IsNotExist(err) {
			// Config file doesn't exist - use defaults
			return cfg, nil
		}
		return nil, err
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}

	// Ensure library path has a default if not specified in config
	if cfg.Library.Path == "" {
		cfg.Library.Path = DefaultLibraryPath
	}

	return cfg, nil
}

// GetLibraryPath returns the configured library path, expanded to an absolute path.
// This is the primary function consumers should use to get the library location.
func GetLibraryPath() string {
	cfg, err := Load()
	if err != nil || cfg == nil {
		return expandPath(DefaultLibraryPath)
	}
	return expandPath(cfg.Library.Path)
}

// GetExecutableDir returns the directory containing the hal9000 executable.
// This is used as the base for project-relative paths (library, services).
// The result is cached after the first call.
func GetExecutableDir() string {
	executableDirOnce.Do(func() {
		execPath, err := os.Executable()
		if err != nil {
			// Fall back to current working directory
			executableDir, _ = os.Getwd()
			return
		}
		// Resolve symlinks to get the real executable location
		execPath, err = filepath.EvalSymlinks(execPath)
		if err != nil {
			executableDir, _ = os.Getwd()
			return
		}
		executableDir = filepath.Dir(execPath)
	})
	return executableDir
}

// expandPath expands ~ to home directory and resolves relative paths.
// Relative paths are resolved from the executable's directory, not the cwd.
func expandPath(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, path[2:])
	}
	// For relative paths, resolve from executable directory
	if !filepath.IsAbs(path) {
		return filepath.Join(GetExecutableDir(), path)
	}
	return path
}

// ResetForTesting resets the global config state. Only use in tests.
func ResetForTesting() {
	configOnce = sync.Once{}
	globalConfig = nil
	configErr = nil
	executableDirOnce = sync.Once{}
	executableDir = ""
}

// SetExecutableDirForTesting allows tests to override the executable directory.
func SetExecutableDirForTesting(dir string) {
	executableDirOnce.Do(func() {
		executableDir = dir
	})
}
