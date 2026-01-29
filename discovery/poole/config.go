package poole

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/pearcec/hal9000/discovery/config"
	"gopkg.in/yaml.v3"
)

// Config holds Poole service configuration.
type Config struct {
	// ActionsPath is the path to the actions.yaml file.
	ActionsPath string `yaml:"actions_path"`
	// DefaultPromptsPath is the path to default prompt templates.
	DefaultPromptsPath string `yaml:"default_prompts_path"`
	// UserPromptsPath is the path to user prompt overrides.
	UserPromptsPath string `yaml:"user_prompts_path"`
	// Enabled controls whether Poole is active.
	Enabled bool `yaml:"enabled"`
}

// DefaultConfig returns the default Poole configuration.
func DefaultConfig() *Config {
	return &Config{
		ActionsPath:        filepath.Join(config.GetConfigDir(), "actions.yaml"),
		DefaultPromptsPath: filepath.Join(config.GetExecutableDir(), "prompts", "defaults"),
		UserPromptsPath:    filepath.Join(config.GetConfigDir(), "prompts"),
		Enabled:            true,
	}
}

// LoadConfig loads Poole configuration from the config directory.
// Falls back to defaults if no config file exists.
func LoadConfig() (*Config, error) {
	cfg := DefaultConfig()

	configPath := filepath.Join(config.GetConfigDir(), "poole.yaml")
	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			// No config file, use defaults
			return cfg, nil
		}
		return nil, fmt.Errorf("failed to read poole config: %w", err)
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("failed to parse poole config: %w", err)
	}

	return cfg, nil
}

// SaveConfig saves Poole configuration to the config directory.
func SaveConfig(cfg *Config) error {
	configPath := filepath.Join(config.GetConfigDir(), "poole.yaml")

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("failed to marshal poole config: %w", err)
	}

	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write poole config: %w", err)
	}

	return nil
}

// GetActionsPath returns the configured path to actions.yaml.
func GetActionsPath() string {
	cfg, err := LoadConfig()
	if err != nil {
		return DefaultConfig().ActionsPath
	}
	return cfg.ActionsPath
}

// GetDefaultPromptsPath returns the path to default prompts.
func GetDefaultPromptsPath() string {
	cfg, err := LoadConfig()
	if err != nil {
		return DefaultConfig().DefaultPromptsPath
	}
	return cfg.DefaultPromptsPath
}

// GetUserPromptsPath returns the path to user prompt overrides.
func GetUserPromptsPath() string {
	cfg, err := LoadConfig()
	if err != nil {
		return DefaultConfig().UserPromptsPath
	}
	return cfg.UserPromptsPath
}
