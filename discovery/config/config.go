// Package config provides configuration management for HAL 9000 discovery components.
// Configuration is loaded from .hal9000/config.yaml in the project directory with sensible defaults.
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
	// DefaultConfigDir is the default directory for HAL 9000 config files.
	// Stored in the project directory (where init was run), not in ~/.config.
	DefaultConfigDir = "./.hal9000"

	// DefaultConfigPath is the default location for the config file.
	DefaultConfigPath = "./.hal9000/config.yaml"

	// DefaultCredentialsDir is the default location for credentials.
	DefaultCredentialsDir = "./.hal9000/credentials"

	// DefaultRuntimeDir is the default location for runtime files (PIDs, logs).
	DefaultRuntimeDir = "./.hal9000/runtime"

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

// GetExecutableDir returns the project root directory for HAL 9000.
// It walks up from the executable's directory looking for a .hal9000/ marker,
// similar to how git finds .git/. Falls back to cwd if not found.
// The result is cached after the first call.
func GetExecutableDir() string {
	executableDirOnce.Do(func() {
		execPath, err := os.Executable()
		if err != nil {
			executableDir, _ = os.Getwd()
			return
		}
		execPath, err = filepath.EvalSymlinks(execPath)
		if err != nil {
			executableDir, _ = os.Getwd()
			return
		}
		executableDir = findProjectRoot(filepath.Dir(execPath))
	})
	return executableDir
}

// findProjectRoot walks up from dir looking for .hal9000/ marker.
// Falls back to cwd if not found.
func findProjectRoot(dir string) string {
	current := dir
	for {
		candidate := filepath.Join(current, ".hal9000", "config.yaml")
		if _, err := os.Stat(candidate); err == nil {
			return current
		}
		parent := filepath.Dir(current)
		if parent == current {
			cwd, _ := os.Getwd()
			return cwd
		}
		current = parent
	}
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

// GetConfigDir returns the absolute path to the config directory.
func GetConfigDir() string {
	return expandPath(DefaultConfigDir)
}

// GetCredentialsDir returns the absolute path to the credentials directory.
func GetCredentialsDir() string {
	return expandPath(DefaultCredentialsDir)
}

// GetRuntimeDir returns the absolute path to the runtime directory.
func GetRuntimeDir() string {
	return expandPath(DefaultRuntimeDir)
}
