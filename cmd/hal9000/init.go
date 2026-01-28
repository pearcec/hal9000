package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

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
  services.yaml     Services configuration (scheduler, floyd watchers)
  credentials/      Credential storage
  services.yaml     Service configuration

Installs CLI to ~/.local/bin/hal9000 (symlink to current executable).

The services.yaml file is pre-configured with:
  - Scheduler enabled (runs scheduled tasks)
  - Floyd watchers disabled (enable after setting up credentials)
  - Absolute paths to binaries based on current directory

After init, run 'hal9000 services start' to start enabled services.

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

	// Create services config file if it doesn't exist
	servicesPath := filepath.Join(configDir, "services.yaml")
	if err := createServicesConfigIfNotExists(servicesPath, &created); err != nil {
		return fmt.Errorf("failed to create services config: %w", err)
	}

	// Create ~/.local/bin and symlink hal9000
	localBinDir := filepath.Join(homeDir, ".local", "bin")
	if err := createDirIfNotExists(localBinDir, &created); err != nil {
		return fmt.Errorf("failed to create ~/.local/bin: %w", err)
	}

	// Create symlink to current executable
	symlinkPath := filepath.Join(localBinDir, "hal9000")
	if err := createSymlinkIfNotExists(symlinkPath, &created); err != nil {
		// Non-fatal - just warn
		fmt.Printf("Warning: could not create symlink: %v\n", err)
	}

	// Check if ~/.local/bin is in PATH
	pathWarning := ""
	if !isInPath(localBinDir) {
		pathWarning = fmt.Sprintf("\nNote: Add ~/.local/bin to your PATH:\n  export PATH=\"%s:$PATH\"\n", localBinDir)
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

	if pathWarning != "" {
		fmt.Print(pathWarning)
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

func createServicesConfigIfNotExists(path string, created *[]string) error {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		// Detect project root (current working directory where init is run)
		projectRoot, err := os.Getwd()
		if err != nil {
			projectRoot = "."
		}

		// Get absolute path for the project root
		projectRoot, err = filepath.Abs(projectRoot)
		if err != nil {
			return fmt.Errorf("failed to get absolute path: %w", err)
		}

		// Build paths to binaries
		hal9000Bin := filepath.Join(projectRoot, "hal9000")
		calendarFloydBin := filepath.Join(projectRoot, "calendar-floyd")
		jiraFloydBin := filepath.Join(projectRoot, "jira-floyd")
		slackFloydBin := filepath.Join(projectRoot, "slack-floyd")

		servicesConfig := fmt.Sprintf(`# HAL 9000 Services Configuration
#
# "I am putting myself to the fullest possible use, which is all I think
# that any conscious entity can ever hope to do."
#
# This file configures HAL's background services.
# Run 'hal9000 services start' to start enabled services.

# Project root where binaries are located
project_root: %s

# HAL Scheduler - runs scheduled tasks
scheduler:
  enabled: true
  binary: %s
  args: ["scheduler", "start"]
  # Runs tasks defined in library/schedules/hal-scheduler.json

# Floyd Watchers - data collection services
# These fetch data from external sources and store in the library.
# Enable as needed after configuring credentials.

floyd:
  calendar:
    enabled: false
    binary: %s
    interval: 15m
    # Requires: ~/.config/hal9000/calendar-floyd-credentials.json

  jira:
    enabled: false
    binary: %s
    interval: 30m
    # Requires: JIRA API token in credentials

  slack:
    enabled: false
    binary: %s
    interval: 5m
    # Requires: Slack bot token in credentials
`, projectRoot, hal9000Bin, calendarFloydBin, jiraFloydBin, slackFloydBin)

		if err := os.WriteFile(path, []byte(servicesConfig), 0644); err != nil {
			return err
		}
		*created = append(*created, path)
	}
	return nil
}

func createSymlinkIfNotExists(symlinkPath string, created *[]string) error {
	// Get current executable path
	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("could not determine executable path: %w", err)
	}

	// Resolve to absolute path (in case it's a symlink itself)
	execPath, err = filepath.EvalSymlinks(execPath)
	if err != nil {
		return fmt.Errorf("could not resolve executable path: %w", err)
	}

	// Check if symlink already exists
	if linkTarget, err := os.Readlink(symlinkPath); err == nil {
		// Symlink exists - check if it points to the right place
		if linkTarget == execPath {
			return nil // Already correct
		}
		// Points somewhere else - remove and recreate
		if err := os.Remove(symlinkPath); err != nil {
			return fmt.Errorf("could not remove old symlink: %w", err)
		}
	} else if _, err := os.Stat(symlinkPath); err == nil {
		// File exists but is not a symlink
		return fmt.Errorf("%s exists and is not a symlink", symlinkPath)
	}

	// Create the symlink
	if err := os.Symlink(execPath, symlinkPath); err != nil {
		return fmt.Errorf("could not create symlink: %w", err)
	}

	*created = append(*created, symlinkPath+" -> "+execPath)
	return nil
}

func isInPath(dir string) bool {
	pathEnv := os.Getenv("PATH")
	paths := strings.Split(pathEnv, string(os.PathListSeparator))
	for _, p := range paths {
		if p == dir {
			return true
		}
	}
	return false
}
