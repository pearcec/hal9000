// Package config provides configuration management for HAL 9000 discovery components.
// Configuration is loaded from ~/.config/hal9000/config.yaml with sensible defaults.
package config

import (
	"os"
	"path/filepath"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"
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

// expandPath expands ~ to home directory and resolves relative paths.
func expandPath(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, path[2:])
	}
	// For relative paths, resolve to absolute
	if !filepath.IsAbs(path) {
		abs, err := filepath.Abs(path)
		if err == nil {
			return abs
		}
	}
	return path
}
