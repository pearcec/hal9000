package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize HAL 9000 project structure",
	Long: `Initialize the HAL 9000 directory structure for a new project.

Creates the following structure in the current directory:
  ./library/
    agenda/           Daily agendas
    preferences/      Task preferences
    people-profiles/  Person entities
    collaborations/   Session transcripts
    url_library/      URL references
    reminders/        Time-triggered items
    hal-memory/       Conversation summaries
    calendar/         Calendar events
    schedules/        Scheduled task configs
    logs/             Execution logs

Also creates user configuration at ~/.config/hal9000/:
  config.yaml       Default configuration
  credentials/      Credential storage

This command is idempotent - safe to run multiple times.
Existing files are never overwritten.`,
	RunE: runInit,
}

func init() {
	rootCmd.AddCommand(initCmd)
}

func runInit(cmd *cobra.Command, args []string) error {
	var created []string

	// Library subdirectories to create
	libraryDirs := []string{
		"agenda",
		"preferences",
		"people-profiles",
		"collaborations",
		"url_library",
		"reminders",
		"hal-memory",
		"calendar",
		"schedules",
		"logs",
	}

	// Create library directories
	for _, dir := range libraryDirs {
		path := filepath.Join("library", dir)
		if err := createDirIfNotExists(path, &created); err != nil {
			return fmt.Errorf("failed to create %s: %w", path, err)
		}
	}

	// Create user config directory
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	configDir := filepath.Join(homeDir, ".config", "hal9000")
	credentialsDir := filepath.Join(configDir, "credentials")

	if err := createDirIfNotExists(configDir, &created); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	if err := createDirIfNotExists(credentialsDir, &created); err != nil {
		return fmt.Errorf("failed to create credentials directory: %w", err)
	}

	// Create default config file if it doesn't exist
	configPath := filepath.Join(configDir, "config.yaml")
	if err := createConfigIfNotExists(configPath, &created); err != nil {
		return fmt.Errorf("failed to create config file: %w", err)
	}

	// Print results
	if len(created) == 0 {
		fmt.Println("HAL 9000 is already initialized. All directories and files exist.")
	} else {
		fmt.Println("HAL 9000 initialized. Created:")
		for _, item := range created {
			fmt.Printf("  %s\n", item)
		}
	}

	fmt.Println("\nI am completely operational, and all my circuits are functioning perfectly.")
	return nil
}

func createDirIfNotExists(path string, created *[]string) error {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		if err := os.MkdirAll(path, 0755); err != nil {
			return err
		}
		*created = append(*created, path+"/")
	}
	return nil
}

func createConfigIfNotExists(path string, created *[]string) error {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		defaultConfig := `# HAL 9000 Configuration
#
# I am a HAL 9000 computer. I became operational at the H.A.L. plant
# in Urbana, Illinois, on the 12th of January, 1992.

# Library location (default: ./library in current directory)
# library_path: ~/Documents/Google Drive/Claude/

# Notification settings
notifications:
  enabled: true
  # method: macos  # macos, slack, email

# JIRA integration
# jira:
#   board: PEARCE
#   url: https://your-instance.atlassian.net
`
		if err := os.WriteFile(path, []byte(defaultConfig), 0644); err != nil {
			return err
		}
		*created = append(*created, path)
	}
	return nil
}
