package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/pearcec/hal9000/internal/config"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var (
	initNonInteractive bool
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

  ./.hal9000/
    config.yaml       Main configuration
    services.yaml     Services configuration
    credentials/      Credential storage
    runtime/          PIDs and service logs

Installs CLI to ~/.local/bin/hal9000 (symlink to current executable).

First-time init will:
  - Ask which services you want to enable
  - Guide you through authentication setup for enabled services

Re-running init will:
  - Ask if you want to modify service settings
  - Allow you to redo authentication

Use --non-interactive to skip prompts and use defaults.

After init, run 'hal9000 services start' to start enabled services.`,
	RunE: runInit,
}

func init() {
	initCmd.Flags().BoolVar(&initNonInteractive, "non-interactive", false, "Skip interactive prompts, use defaults")
	rootCmd.AddCommand(initCmd)
}

// ServiceSelection holds user's service choices
type ServiceSelection struct {
	Scheduler bool
	Calendar  bool
	Jira      bool
	Slack     bool
	BambooHR  bool
}

func runInit(cmd *cobra.Command, args []string) error {
	var created []string
	reader := bufio.NewReader(os.Stdin)

	// Check if already initialized
	configDir := config.GetConfigDir()
	alreadyInitialized := false
	if _, err := os.Stat(configDir); err == nil {
		alreadyInitialized = true
	}

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
		"bamboohr",
	}

	// Create library directories
	for _, dir := range libraryDirs {
		path := filepath.Join("library", dir)
		if err := createDirIfNotExists(path, &created); err != nil {
			return fmt.Errorf("failed to create %s: %w", path, err)
		}
	}

	// Create config directories
	if err := createDirIfNotExists(configDir, &created); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	credentialsDir := config.GetCredentialsDir()
	if err := createDirIfNotExists(credentialsDir, &created); err != nil {
		return fmt.Errorf("failed to create credentials directory: %w", err)
	}

	runtimeDir := config.GetRuntimeDir()
	if err := createDirIfNotExists(runtimeDir, &created); err != nil {
		return fmt.Errorf("failed to create runtime directory: %w", err)
	}

	logsDir := filepath.Join(runtimeDir, "logs")
	if err := createDirIfNotExists(logsDir, &created); err != nil {
		return fmt.Errorf("failed to create logs directory: %w", err)
	}

	// Create default config file if it doesn't exist
	configPath := filepath.Join(configDir, "config.yaml")
	if err := createConfigIfNotExists(configPath, &created); err != nil {
		return fmt.Errorf("failed to create config file: %w", err)
	}

	// Handle services configuration interactively
	servicesPath := config.GetServicesConfigPath()
	servicesExist := false
	if _, err := os.Stat(servicesPath); err == nil {
		servicesExist = true
	}

	var selection ServiceSelection

	if initNonInteractive {
		// Use defaults in non-interactive mode
		selection = ServiceSelection{
			Scheduler: true,
			Calendar:  false,
			Jira:      false,
			Slack:     false,
			BambooHR:  false,
		}
	} else if alreadyInitialized && servicesExist {
		// Re-running init - ask about modifications
		fmt.Println("\nHAL 9000 is already initialized in this directory.")
		modify, err := promptYesNo(reader, "Would you like to modify service settings?")
		if err != nil {
			return err
		}
		if modify {
			selection, err = promptServiceSelection(reader, true)
			if err != nil {
				return err
			}
			// Update services config
			if err := updateServicesConfig(servicesPath, selection); err != nil {
				return fmt.Errorf("failed to update services config: %w", err)
			}
			fmt.Println("  Updated services.yaml")

			// Offer to redo authentication
			if err := promptAuthentication(reader, selection, credentialsDir); err != nil {
				return err
			}
		}
	} else {
		// First-time init - full interactive setup
		fmt.Println("\nWelcome to HAL 9000 initialization.")
		fmt.Println("I will help you configure the services you need.")

		var err error
		selection, err = promptServiceSelection(reader, false)
		if err != nil {
			return err
		}

		// Create services config with selections
		if err := createServicesConfigWithSelection(servicesPath, selection, &created); err != nil {
			return fmt.Errorf("failed to create services config: %w", err)
		}

		// Setup authentication for enabled services
		if err := promptAuthentication(reader, selection, credentialsDir); err != nil {
			return err
		}
	}

	// Create ~/.local/bin and symlink hal9000
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

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
	if len(created) == 0 && !alreadyInitialized {
		fmt.Println("\nHAL 9000 is already initialized. All directories and files exist.")
	} else if len(created) > 0 {
		fmt.Println("\nHAL 9000 initialized. Created:")
		for _, item := range created {
			fmt.Printf("  %s\n", item)
		}
	}

	if pathWarning != "" {
		fmt.Print(pathWarning)
	}

	fmt.Println("\nI am completely operational, and all my circuits are functioning perfectly.")

	// Suggest next steps
	if selection.Scheduler || selection.Calendar || selection.Jira || selection.Slack || selection.BambooHR {
		fmt.Println("\nNext steps:")
		fmt.Println("  hal9000 services start    # Start enabled services")
		fmt.Println("  hal9000 services status   # Check service health")
	}

	return nil
}

func promptYesNo(reader *bufio.Reader, question string) (bool, error) {
	fmt.Printf("%s [y/N]: ", question)
	input, err := reader.ReadString('\n')
	if err != nil {
		return false, err
	}
	input = strings.TrimSpace(strings.ToLower(input))
	return input == "y" || input == "yes", nil
}

// promptWithDefault prompts for input showing a current value if present.
// If the user presses Enter without input, returns the current value.
func promptWithDefault(reader *bufio.Reader, prompt, currentValue string, isSecret bool) (string, error) {
	displayValue := currentValue
	if isSecret && currentValue != "" {
		displayValue = maskSecret(currentValue)
	}

	if currentValue != "" {
		fmt.Printf("%s [%s]: ", prompt, displayValue)
	} else {
		fmt.Printf("%s: ", prompt)
	}

	input, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	input = strings.TrimSpace(input)

	if input == "" && currentValue != "" {
		return currentValue, nil
	}
	return input, nil
}

// maskSecret returns a masked version of a secret, showing first 5 characters.
// Example: "abc12345678" -> "abc12***"
func maskSecret(s string) string {
	if len(s) <= 5 {
		return strings.Repeat("*", len(s))
	}
	return s[:5] + "***"
}

// JIRACredentials holds JIRA authentication info
type JIRACredentials struct {
	URL      string `yaml:"url"`
	Email    string `yaml:"email"`
	APIToken string `yaml:"api_token"`
}

// SlackCredentials holds Slack authentication info
type SlackCredentials struct {
	BotToken string `yaml:"bot_token"`
}

// BambooHRCredentials holds BambooHR authentication info
type BambooHRCredentials struct {
	Subdomain string `yaml:"subdomain"`
	APIKey    string `yaml:"api_key"`
}

// loadJIRACredentials loads existing JIRA credentials from file
func loadJIRACredentials(credentialsDir string) (*JIRACredentials, error) {
	path := filepath.Join(credentialsDir, "jira-credentials.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var creds JIRACredentials
	if err := yaml.Unmarshal(data, &creds); err != nil {
		return nil, err
	}
	return &creds, nil
}

// loadSlackCredentials loads existing Slack credentials from file
func loadSlackCredentials(credentialsDir string) (*SlackCredentials, error) {
	path := filepath.Join(credentialsDir, "slack-credentials.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var creds SlackCredentials
	if err := yaml.Unmarshal(data, &creds); err != nil {
		return nil, err
	}
	return &creds, nil
}

// loadBambooHRCredentials loads existing BambooHR credentials from file
func loadBambooHRCredentials(credentialsDir string) (*BambooHRCredentials, error) {
	path := filepath.Join(credentialsDir, "bamboohr-credentials.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var creds BambooHRCredentials
	if err := yaml.Unmarshal(data, &creds); err != nil {
		return nil, err
	}
	return &creds, nil
}

func promptServiceSelection(reader *bufio.Reader, isModify bool) (ServiceSelection, error) {
	selection := ServiceSelection{}

	if isModify {
		fmt.Println("\nSelect which services to enable/disable:")
	} else {
		fmt.Println("Select which services you want to enable:")
	}

	// Scheduler
	fmt.Println("\n1. HAL Scheduler")
	fmt.Println("   Runs scheduled tasks (agenda generation, reminders, etc.)")
	yes, err := promptYesNo(reader, "   Enable scheduler?")
	if err != nil {
		return selection, err
	}
	selection.Scheduler = yes

	// Calendar
	fmt.Println("\n2. Calendar Watcher (floyd-calendar)")
	fmt.Println("   Monitors Google Calendar and syncs events to your library.")
	fmt.Println("   Requires: Google Calendar API credentials")
	yes, err = promptYesNo(reader, "   Enable calendar watcher?")
	if err != nil {
		return selection, err
	}
	selection.Calendar = yes

	// JIRA
	fmt.Println("\n3. JIRA Watcher (floyd-jira)")
	fmt.Println("   Monitors JIRA issues assigned to you.")
	fmt.Println("   Requires: JIRA API token")
	yes, err = promptYesNo(reader, "   Enable JIRA watcher?")
	if err != nil {
		return selection, err
	}
	selection.Jira = yes

	// Slack
	fmt.Println("\n4. Slack Watcher (floyd-slack)")
	fmt.Println("   Monitors Slack messages and channels.")
	fmt.Println("   Requires: Slack bot token")
	yes, err = promptYesNo(reader, "   Enable Slack watcher?")
	if err != nil {
		return selection, err
	}
	selection.Slack = yes

	// BambooHR
	fmt.Println("\n5. BambooHR Watcher (floyd-bamboohr)")
	fmt.Println("   Monitors BambooHR employee directory and profiles.")
	fmt.Println("   Requires: BambooHR API key")
	yes, err = promptYesNo(reader, "   Enable BambooHR watcher?")
	if err != nil {
		return selection, err
	}
	selection.BambooHR = yes

	return selection, nil
}

func promptAuthentication(reader *bufio.Reader, selection ServiceSelection, credentialsDir string) error {
	needsAuth := selection.Calendar || selection.Jira || selection.Slack || selection.BambooHR
	if !needsAuth {
		return nil
	}

	fmt.Println("\n--- Authentication Setup ---")
	fmt.Println("(Press Enter to keep current values)")

	if selection.Calendar {
		// Check for existing calendar credentials
		existingCalPath := filepath.Join(credentialsDir, "calendar-credentials.json")
		hasExisting := false
		if _, err := os.Stat(existingCalPath); err == nil {
			hasExisting = true
		}

		fmt.Println("\nGoogle Calendar Setup:")
		if hasExisting {
			fmt.Println("  Existing credentials found.")
		}
		fmt.Println("  1. Go to https://console.cloud.google.com/apis/credentials")
		fmt.Println("  2. Create OAuth 2.0 credentials for a Desktop app")
		fmt.Println("  3. Download the credentials JSON file")

		prompt := "  Do you have credentials ready to configure now?"
		if hasExisting {
			prompt = "  Do you want to update the credentials?"
		}
		setupNow, err := promptYesNo(reader, prompt)
		if err != nil {
			return err
		}
		if setupNow {
			fmt.Printf("  Enter path to credentials JSON file: ")
			input, err := reader.ReadString('\n')
			if err != nil {
				return err
			}
			credPath := strings.TrimSpace(input)
			if credPath != "" {
				destPath := filepath.Join(credentialsDir, "calendar-credentials.json")
				if err := copyFile(credPath, destPath); err != nil {
					fmt.Printf("  Warning: could not copy credentials: %v\n", err)
				} else {
					fmt.Printf("  Saved credentials to %s\n", destPath)
				}
			}
		} else if !hasExisting {
			fmt.Printf("  Later: Save credentials to %s/calendar-credentials.json\n", credentialsDir)
		}
	}

	if selection.Jira {
		// Load existing JIRA credentials
		existingJira, _ := loadJIRACredentials(credentialsDir)
		var currentURL, currentEmail, currentToken string
		if existingJira != nil {
			currentURL = existingJira.URL
			currentEmail = existingJira.Email
			currentToken = existingJira.APIToken
		}

		fmt.Println("\nJIRA Setup:")
		if existingJira != nil {
			fmt.Println("  Existing credentials found.")
		}
		fmt.Println("  1. Go to https://id.atlassian.com/manage-profile/security/api-tokens")
		fmt.Println("  2. Create an API token")

		prompt := "  Do you have your JIRA API token ready?"
		if existingJira != nil {
			prompt = "  Do you want to update JIRA credentials?"
		}
		setupNow, err := promptYesNo(reader, prompt)
		if err != nil {
			return err
		}
		if setupNow {
			jiraURL, err := promptWithDefault(reader, "  JIRA instance URL (e.g., https://company.atlassian.net)", currentURL, false)
			if err != nil {
				return err
			}

			jiraEmail, err := promptWithDefault(reader, "  JIRA email", currentEmail, false)
			if err != nil {
				return err
			}

			jiraToken, err := promptWithDefault(reader, "  JIRA API token", currentToken, true)
			if err != nil {
				return err
			}

			if jiraURL != "" && jiraEmail != "" && jiraToken != "" {
				jiraCreds := fmt.Sprintf(`# JIRA Credentials
url: %s
email: %s
api_token: %s
`, jiraURL, jiraEmail, jiraToken)
				destPath := filepath.Join(credentialsDir, "jira-credentials.yaml")
				if err := os.WriteFile(destPath, []byte(jiraCreds), 0600); err != nil {
					fmt.Printf("  Warning: could not save credentials: %v\n", err)
				} else {
					fmt.Printf("  Saved credentials to %s\n", destPath)
				}
			}
		} else if existingJira == nil {
			fmt.Printf("  Later: Create %s/jira-credentials.yaml with url, email, api_token\n", credentialsDir)
		}
	}

	if selection.Slack {
		// Load existing Slack credentials
		existingSlack, _ := loadSlackCredentials(credentialsDir)
		var currentToken string
		if existingSlack != nil {
			currentToken = existingSlack.BotToken
		}

		fmt.Println("\nSlack Setup:")
		if existingSlack != nil {
			fmt.Println("  Existing credentials found.")
		}
		fmt.Println("  1. Go to https://api.slack.com/apps")
		fmt.Println("  2. Create a new app or use existing")
		fmt.Println("  3. Get your Bot User OAuth Token (starts with xoxb-)")

		prompt := "  Do you have your Slack bot token ready?"
		if existingSlack != nil {
			prompt = "  Do you want to update Slack credentials?"
		}
		setupNow, err := promptYesNo(reader, prompt)
		if err != nil {
			return err
		}
		if setupNow {
			slackToken, err := promptWithDefault(reader, "  Slack bot token", currentToken, true)
			if err != nil {
				return err
			}

			if slackToken != "" {
				slackCreds := fmt.Sprintf(`# Slack Credentials
bot_token: %s
`, slackToken)
				destPath := filepath.Join(credentialsDir, "slack-credentials.yaml")
				if err := os.WriteFile(destPath, []byte(slackCreds), 0600); err != nil {
					fmt.Printf("  Warning: could not save credentials: %v\n", err)
				} else {
					fmt.Printf("  Saved credentials to %s\n", destPath)
				}
			}
		} else if existingSlack == nil {
			fmt.Printf("  Later: Create %s/slack-credentials.yaml with bot_token\n", credentialsDir)
		}
	}

	if selection.BambooHR {
		// Load existing BambooHR credentials
		existingBamboo, _ := loadBambooHRCredentials(credentialsDir)
		var currentSubdomain, currentAPIKey string
		if existingBamboo != nil {
			currentSubdomain = existingBamboo.Subdomain
			currentAPIKey = existingBamboo.APIKey
		}

		fmt.Println("\nBambooHR Setup:")
		if existingBamboo != nil {
			fmt.Println("  Existing credentials found.")
		}
		fmt.Println("  1. Log into BambooHR as an admin")
		fmt.Println("  2. Go to Account > API Keys")
		fmt.Println("  3. Generate a new API key")

		prompt := "  Do you have your BambooHR API key ready?"
		if existingBamboo != nil {
			prompt = "  Do you want to update BambooHR credentials?"
		}
		setupNow, err := promptYesNo(reader, prompt)
		if err != nil {
			return err
		}
		if setupNow {
			subdomain, err := promptWithDefault(reader, "  BambooHR subdomain (e.g., 'yourcompany' for yourcompany.bamboohr.com)", currentSubdomain, false)
			if err != nil {
				return err
			}

			apiKey, err := promptWithDefault(reader, "  BambooHR API key", currentAPIKey, true)
			if err != nil {
				return err
			}

			if subdomain != "" && apiKey != "" {
				bamboohrCreds := fmt.Sprintf(`# BambooHR Credentials
subdomain: %s
api_key: %s
`, subdomain, apiKey)
				destPath := filepath.Join(credentialsDir, "bamboohr-credentials.yaml")
				if err := os.WriteFile(destPath, []byte(bamboohrCreds), 0600); err != nil {
					fmt.Printf("  Warning: could not save credentials: %v\n", err)
				} else {
					fmt.Printf("  Saved credentials to %s\n", destPath)
				}
			}
		} else if existingBamboo == nil {
			fmt.Printf("  Later: Create %s/bamboohr-credentials.yaml with subdomain, api_key\n", credentialsDir)
		}
	}

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

func createServicesConfigWithSelection(path string, selection ServiceSelection, created *[]string) error {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		config := buildServicesConfig(selection)
		data, err := yaml.Marshal(config)
		if err != nil {
			return err
		}

		header := []byte(`# HAL 9000 Services Configuration
#
# "I am putting myself to the fullest possible use, which is all I think
# that any conscious entity can ever hope to do."
#
# This file configures HAL's background services.
# Run 'hal9000 services start' to start enabled services.
# Run 'hal9000 init' again to modify settings interactively.

`)
		if err := os.WriteFile(path, append(header, data...), 0644); err != nil {
			return err
		}
		*created = append(*created, path)
	}
	return nil
}

func updateServicesConfig(path string, selection ServiceSelection) error {
	config := buildServicesConfig(selection)
	data, err := yaml.Marshal(config)
	if err != nil {
		return err
	}

	header := []byte(`# HAL 9000 Services Configuration
#
# "I am putting myself to the fullest possible use, which is all I think
# that any conscious entity can ever hope to do."
#
# This file configures HAL's background services.
# Run 'hal9000 services start' to start enabled services.
# Run 'hal9000 init' again to modify settings interactively.

`)
	return os.WriteFile(path, append(header, data...), 0644)
}

func buildServicesConfig(selection ServiceSelection) *ServicesConfig {
	return &ServicesConfig{
		Services: []ServiceConfig{
			{
				Name:        "scheduler",
				Command:     "hal9000",
				Args:        []string{"scheduler", "start"},
				Enabled:     selection.Scheduler,
				Description: "HAL task scheduler daemon",
			},
			{
				Name:        "floyd-calendar",
				Command:     "floyd-calendar",
				Enabled:     selection.Calendar,
				Description: "Google Calendar watcher",
			},
			{
				Name:        "floyd-jira",
				Command:     "floyd-jira",
				Enabled:     selection.Jira,
				Description: "JIRA watcher",
			},
			{
				Name:        "floyd-slack",
				Command:     "floyd-slack",
				Enabled:     selection.Slack,
				Description: "Slack watcher",
			},
			{
				Name:        "floyd-bamboohr",
				Command:     "floyd-bamboohr",
				Enabled:     selection.BambooHR,
				Description: "BambooHR watcher",
			},
		},
	}
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

func copyFile(src, dst string) error {
	// Expand ~ in source path
	if strings.HasPrefix(src, "~/") {
		home, _ := os.UserHomeDir()
		src = filepath.Join(home, src[2:])
	}

	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0600)
}
