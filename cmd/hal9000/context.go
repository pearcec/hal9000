package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"github.com/pearcec/hal9000/cmd/hal9000/tasks"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

const (
	configDir        = "~/.config/hal9000"
	servicesFile     = "services.yaml"
	defaultLibPath   = "~/Documents/Google Drive/Claude"
)

// ServiceConfig represents a service configuration entry.
type ServiceConfig struct {
	Name        string   `yaml:"name"`
	Command     string   `yaml:"command"`
	Args        []string `yaml:"args,omitempty"`
	Enabled     bool     `yaml:"enabled"`
	Description string   `yaml:"description"`
}

// ServicesConfig represents the services.yaml structure.
type ServicesConfig struct {
	Services []ServiceConfig `yaml:"services"`
}

// IntegrationStatus represents the status of an integration.
type IntegrationStatus struct {
	Name        string
	Configured  bool
	Description string
}

var contextCmd = &cobra.Command{
	Use:   "context",
	Short: "Output HAL 9000 capabilities for Claude context injection",
	Long: `Output all HAL 9000 capabilities in XML format suitable for Claude's context window.
"I am putting myself to the fullest possible use, which is all I think
that any conscious entity can ever hope to do."

This command provides Claude with dynamic, up-to-date knowledge of:
  - Available commands and their descriptions
  - Library folder structure
  - Live service status (running/stopped)
  - Configured integrations

Example:
  hal9000 context`,
	Run: runContext,
}

func init() {
	rootCmd.AddCommand(contextCmd)
}

func runContext(cmd *cobra.Command, args []string) {
	fmt.Println("<hal9000-capabilities>")
	fmt.Println()

	// Section 1: Overview
	printOverview()

	// Section 2: Commands
	printCommands()

	// Section 3: Tasks
	printTasks()

	// Section 4: Library Structure
	printLibraryStructure()

	// Section 5: Service Status
	printServiceStatus()

	// Section 6: Integration Status
	printIntegrationStatus()

	fmt.Println("</hal9000-capabilities>")
}

func printOverview() {
	fmt.Println("<overview>")
	fmt.Println("HAL 9000 is a personal AI assistant system that helps with:")
	fmt.Println("- Task scheduling and automation")
	fmt.Println("- Meeting transcript processing")
	fmt.Println("- Personal knowledge management via the Library")
	fmt.Println("- Calendar and productivity integrations")
	fmt.Println("</overview>")
	fmt.Println()
}

func printCommands() {
	fmt.Println("<commands>")

	// Get all commands from rootCmd
	for _, cmd := range rootCmd.Commands() {
		if cmd.Hidden {
			continue
		}

		fmt.Printf("  <command name=%q>\n", cmd.Name())
		fmt.Printf("    <description>%s</description>\n", cmd.Short)

		// Print subcommands if any
		subCmds := cmd.Commands()
		if len(subCmds) > 0 {
			fmt.Println("    <subcommands>")
			for _, sub := range subCmds {
				if sub.Hidden {
					continue
				}
				fmt.Printf("      <subcommand name=%q>%s</subcommand>\n", sub.Name(), sub.Short)
			}
			fmt.Println("    </subcommands>")
		}

		fmt.Println("  </command>")
	}

	fmt.Println("</commands>")
	fmt.Println()
}

func printTasks() {
	taskList := tasks.List()
	if len(taskList) == 0 {
		return
	}

	fmt.Println("<tasks>")
	fmt.Println("  <!-- Tasks are specialized routines that can be scheduled or run manually -->")

	for _, task := range taskList {
		fmt.Printf("  <task name=%q>\n", task.Name())
		fmt.Printf("    <description>%s</description>\n", task.Description())

		questions := task.SetupQuestions()
		if len(questions) > 0 {
			fmt.Println("    <setup-options>")
			for _, q := range questions {
				fmt.Printf("      <option key=%q default=%q>%s</option>\n",
					q.Key, q.Default, q.Question)
			}
			fmt.Println("    </setup-options>")
		}

		fmt.Println("  </task>")
	}

	fmt.Println("</tasks>")
	fmt.Println()
}

func printLibraryStructure() {
	fmt.Println("<library>")
	fmt.Printf("  <path>%s</path>\n", defaultLibPath)
	fmt.Println("  <folders>")
	fmt.Println("    <folder name=\"calendar\">Calendar events and meeting data</folder>")
	fmt.Println("    <folder name=\"people\">People profiles from 1:1 meetings</folder>")
	fmt.Println("    <folder name=\"collaborations\">Team/collaboration records</folder>")
	fmt.Println("    <folder name=\"preferences\">Task preferences and settings</folder>")
	fmt.Println("    <folder name=\"schedules\">Scheduler configuration</folder>")
	fmt.Println("    <folder name=\"logs\">Application logs</folder>")
	fmt.Println("  </folders>")
	fmt.Println("  <usage>")
	fmt.Println("    Use 'hal9000 library read <type>/<name>' to read entities")
	fmt.Println("    Use 'hal9000 library list' to see all entity types")
	fmt.Println("    Use 'hal9000 library query --type=<type>' to search")
	fmt.Println("  </usage>")
	fmt.Println("</library>")
	fmt.Println()
}

func printServiceStatus() {
	fmt.Println("<services>")

	services := loadServicesConfig()

	for _, svc := range services {
		status := getServiceStatus(svc)
		enabledStr := "false"
		if svc.Enabled {
			enabledStr = "true"
		}

		fmt.Printf("  <service name=%q enabled=%q status=%q>\n",
			svc.Name, enabledStr, status)
		fmt.Printf("    <description>%s</description>\n", svc.Description)
		fmt.Println("  </service>")
	}

	fmt.Println("</services>")
	fmt.Println()
}

func printIntegrationStatus() {
	fmt.Println("<integrations>")

	integrations := checkIntegrations()

	for _, integration := range integrations {
		configuredStr := "false"
		if integration.Configured {
			configuredStr = "true"
		}

		fmt.Printf("  <integration name=%q configured=%q>\n",
			integration.Name, configuredStr)
		fmt.Printf("    <description>%s</description>\n", integration.Description)
		fmt.Println("  </integration>")
	}

	fmt.Println("</integrations>")
	fmt.Println()
}

// loadServicesConfig loads the services configuration from services.yaml.
func loadServicesConfig() []ServiceConfig {
	configPath := filepath.Join(expandContextPath(configDir), servicesFile)

	data, err := os.ReadFile(configPath)
	if err != nil {
		// Return default services if file doesn't exist
		return []ServiceConfig{
			{Name: "scheduler", Description: "HAL task scheduler daemon", Enabled: false},
		}
	}

	var config ServicesConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return []ServiceConfig{}
	}

	return config.Services
}

// getServiceStatus checks if a service is currently running.
func getServiceStatus(svc ServiceConfig) string {
	switch svc.Name {
	case "scheduler":
		return getSchedulerStatus()
	default:
		// For other services, check if there's a PID file
		pidPath := filepath.Join(expandContextPath(configDir), svc.Name+".pid")
		if pid, err := readPIDFile(pidPath); err == nil {
			if isProcessAlive(pid) {
				return "running"
			}
		}
		return "stopped"
	}
}

// getSchedulerStatus checks the scheduler daemon status.
func getSchedulerStatus() string {
	// Check multiple possible PID file locations
	pidPaths := []string{
		filepath.Join(expandContextPath(configDir), "scheduler.pid"),
		expandContextPath("~/.hal9000/scheduler.pid"),
	}

	for _, pidPath := range pidPaths {
		if pid, err := readPIDFile(pidPath); err == nil {
			if isProcessAlive(pid) {
				return "running"
			}
		}
	}

	return "stopped"
}

// readPIDFile reads a PID from a file.
func readPIDFile(path string) (int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(strings.TrimSpace(string(data)))
}

// isProcessAlive checks if a process is running.
func isProcessAlive(pid int) bool {
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	// On Unix, FindProcess always succeeds; send signal 0 to check
	return process.Signal(syscall.Signal(0)) == nil
}

// checkIntegrations checks which integrations are configured.
func checkIntegrations() []IntegrationStatus {
	integrations := []IntegrationStatus{
		{
			Name:        "calendar",
			Description: "Google Calendar integration for fetching events and transcripts",
			Configured:  checkCalendarIntegration(),
		},
		{
			Name:        "jira",
			Description: "JIRA integration for issue tracking",
			Configured:  checkJiraIntegration(),
		},
		{
			Name:        "slack",
			Description: "Slack integration for messaging",
			Configured:  checkSlackIntegration(),
		},
	}

	return integrations
}

// checkCalendarIntegration checks if calendar credentials exist.
func checkCalendarIntegration() bool {
	credPaths := []string{
		filepath.Join(expandContextPath(configDir), "calendar-floyd-credentials.json"),
		filepath.Join(expandContextPath(configDir), "calendar-floyd-token.json"),
	}

	for _, path := range credPaths {
		if _, err := os.Stat(path); err == nil {
			return true
		}
	}
	return false
}

// checkJiraIntegration checks if JIRA credentials exist.
func checkJiraIntegration() bool {
	credPaths := []string{
		filepath.Join(expandContextPath(configDir), "jira-credentials.json"),
		filepath.Join(expandContextPath(configDir), "jira-token.json"),
		filepath.Join(expandContextPath(configDir), "credentials", "jira.json"),
	}

	for _, path := range credPaths {
		if _, err := os.Stat(path); err == nil {
			return true
		}
	}
	return false
}

// checkSlackIntegration checks if Slack credentials exist.
func checkSlackIntegration() bool {
	credPaths := []string{
		filepath.Join(expandContextPath(configDir), "slack-credentials.json"),
		filepath.Join(expandContextPath(configDir), "slack-token.json"),
		filepath.Join(expandContextPath(configDir), "credentials", "slack.json"),
	}

	for _, path := range credPaths {
		if _, err := os.Stat(path); err == nil {
			return true
		}
	}
	return false
}

// expandContextPath expands ~ to home directory.
func expandContextPath(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, path[2:])
	}
	return path
}
