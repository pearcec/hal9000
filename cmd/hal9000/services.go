package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var (
	servicesQuiet     bool
	servicesCheckOnly bool
	servicesJSONOut   bool
	servicesLogTail   int
)

// ServiceConfig represents a single service configuration
type ServiceConfig struct {
	Name        string `yaml:"name"`
	Command     string `yaml:"command"`
	Args        []string `yaml:"args,omitempty"`
	Enabled     bool   `yaml:"enabled"`
	Description string `yaml:"description,omitempty"`
}

// ServicesConfig holds all service configurations
type ServicesConfig struct {
	Services []ServiceConfig `yaml:"services"`
}

// ServiceStatus represents the runtime status of a service
type ServiceStatus struct {
	Name        string `json:"name"`
	Running     bool   `json:"running"`
	PID         int    `json:"pid,omitempty"`
	Uptime      string `json:"uptime,omitempty"`
	Description string `json:"description,omitempty"`
	Enabled     bool   `json:"enabled"`
}

var servicesCmd = &cobra.Command{
	Use:   "services",
	Short: "Manage HAL 9000 background services",
	Long: `Unified management for HAL 9000 background services.
"I am putting myself to the fullest possible use, which is all I think
that any conscious entity can ever hope to do."

Services:
  scheduler        HAL task scheduler daemon
  floyd-calendar   Google Calendar watcher
  floyd-jira       JIRA watcher
  floyd-slack      Slack watcher

Commands:
  start [service]    Start all or specific service
  stop [service]     Stop all or specific service
  status             Show service health
  restart [service]  Restart services
  logs [service]     View service logs`,
}

var servicesStartCmd = &cobra.Command{
	Use:   "start [service]",
	Short: "Start all or specific service",
	Long: `Start HAL 9000 services. Without arguments, starts all enabled services.
With a service name, starts only that service.

Examples:
  hal9000 services start              # Start all enabled services
  hal9000 services start scheduler    # Start only scheduler
  hal9000 services start floyd-calendar`,
	RunE: runServicesStart,
}

var servicesStopCmd = &cobra.Command{
	Use:   "stop [service]",
	Short: "Stop all or specific service",
	Long: `Stop HAL 9000 services. Without arguments, stops all running services.
With a service name, stops only that service.

Examples:
  hal9000 services stop               # Stop all services
  hal9000 services stop scheduler     # Stop only scheduler`,
	RunE: runServicesStop,
}

var servicesStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show service health status",
	Long: `Display the status of all HAL 9000 services.

Use --quiet for scripts/hooks (exits 0 if healthy, 1 if problems).
Use --json for machine-readable output.

Examples:
  hal9000 services status
  hal9000 services status --quiet
  hal9000 services status --json`,
	RunE: runServicesStatus,
}

var servicesRestartCmd = &cobra.Command{
	Use:   "restart [service]",
	Short: "Restart all or specific service",
	Long: `Restart HAL 9000 services. Equivalent to stop followed by start.

Examples:
  hal9000 services restart              # Restart all services
  hal9000 services restart scheduler    # Restart only scheduler`,
	RunE: runServicesRestart,
}

var servicesLogsCmd = &cobra.Command{
	Use:   "logs [service]",
	Short: "View service logs",
	Long: `View logs from HAL 9000 services.

Without arguments, shows logs from all services.
With a service name, shows logs only from that service.

Examples:
  hal9000 services logs                 # Show all service logs
  hal9000 services logs scheduler       # Show scheduler logs only
  hal9000 services logs --tail=100      # Show last 100 lines`,
	RunE: runServicesLogs,
}

var servicesDiagnoseCmd = &cobra.Command{
	Use:   "diagnose [service]",
	Short: "Diagnose service issues",
	Long: `Diagnose problems with HAL 9000 services.

Without arguments, diagnoses all services.
With a service name, focuses on that specific service.

Checks performed:
  - Service configuration validity
  - Executable existence and permissions
  - Process status and PID files
  - Recent log entries for errors

Examples:
  hal9000 services diagnose              # Diagnose all services
  hal9000 services diagnose floyd-calendar`,
	RunE: runServicesDiagnose,
}

func init() {
	// Status flags
	servicesStatusCmd.Flags().BoolVarP(&servicesQuiet, "quiet", "q", false, "Quiet mode (exit 0 if healthy, 1 if problems)")
	servicesStatusCmd.Flags().BoolVar(&servicesCheckOnly, "check-only", false, "Silent check (exit 0 if healthy, 1 if problems, no output)")
	servicesStatusCmd.Flags().BoolVar(&servicesJSONOut, "json", false, "Output as JSON")

	// Logs flags
	servicesLogsCmd.Flags().IntVar(&servicesLogTail, "tail", 50, "Number of lines to show")

	// Add subcommands
	servicesCmd.AddCommand(servicesStartCmd)
	servicesCmd.AddCommand(servicesStopCmd)
	servicesCmd.AddCommand(servicesStatusCmd)
	servicesCmd.AddCommand(servicesRestartCmd)
	servicesCmd.AddCommand(servicesLogsCmd)
	servicesCmd.AddCommand(servicesDiagnoseCmd)

	// Register with root command
	rootCmd.AddCommand(servicesCmd)
}

// Path helpers

func getServicesConfigPath() string {
	return expandPath("~/.config/hal9000/services.yaml")
}

func getServicePIDPath(serviceName string) string {
	return expandPath(fmt.Sprintf("~/.config/hal9000/%s.pid", serviceName))
}

func getServiceLogPath(serviceName string) string {
	return expandPath(fmt.Sprintf("~/.config/hal9000/logs/%s.log", serviceName))
}

// Config management

func loadServicesConfig() (*ServicesConfig, error) {
	configPath := getServicesConfigPath()
	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			// Return default config if file doesn't exist
			return getDefaultServicesConfig(), nil
		}
		return nil, err
	}

	var config ServicesConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}
	return &config, nil
}

func getDefaultServicesConfig() *ServicesConfig {
	execPath, _ := os.Executable()
	return &ServicesConfig{
		Services: []ServiceConfig{
			{
				Name:        "scheduler",
				Command:     execPath,
				Args:        []string{"scheduler", "start"},
				Enabled:     true,
				Description: "HAL task scheduler daemon",
			},
			{
				Name:        "floyd-calendar",
				Command:     "floyd-calendar",
				Enabled:     false,
				Description: "Google Calendar watcher",
			},
			{
				Name:        "floyd-jira",
				Command:     "floyd-jira",
				Enabled:     false,
				Description: "JIRA watcher",
			},
			{
				Name:        "floyd-slack",
				Command:     "floyd-slack",
				Enabled:     false,
				Description: "Slack watcher",
			},
		},
	}
}

func saveServicesConfig(config *ServicesConfig) error {
	configPath := getServicesConfigPath()
	if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
		return err
	}

	data, err := yaml.Marshal(config)
	if err != nil {
		return err
	}

	header := []byte(`# HAL 9000 Services Configuration
# Edit this file to enable/disable services and customize commands.
#
# Services:
#   scheduler        HAL task scheduler daemon
#   floyd-calendar   Google Calendar watcher
#   floyd-jira       JIRA watcher
#   floyd-slack      Slack watcher

`)
	return os.WriteFile(configPath, append(header, data...), 0644)
}

func findService(config *ServicesConfig, name string) *ServiceConfig {
	for i := range config.Services {
		if config.Services[i].Name == name {
			return &config.Services[i]
		}
	}
	return nil
}

// PID file management

func writeServicePID(serviceName string, pid int) error {
	pidPath := getServicePIDPath(serviceName)
	if err := os.MkdirAll(filepath.Dir(pidPath), 0755); err != nil {
		return err
	}
	return os.WriteFile(pidPath, []byte(strconv.Itoa(pid)), 0644)
}

func readServicePID(serviceName string) (int, error) {
	data, err := os.ReadFile(getServicePIDPath(serviceName))
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(strings.TrimSpace(string(data)))
}

func removeServicePID(serviceName string) error {
	return os.Remove(getServicePIDPath(serviceName))
}

func isServiceRunning(serviceName string) (bool, int) {
	pid, err := readServicePID(serviceName)
	if err != nil {
		return false, 0
	}

	process, err := os.FindProcess(pid)
	if err != nil {
		return false, 0
	}

	// On Unix, FindProcess always succeeds; we need to send signal 0 to check
	err = process.Signal(syscall.Signal(0))
	if err != nil {
		removeServicePID(serviceName)
		return false, 0
	}

	return true, pid
}

func getServiceUptime(serviceName string) string {
	pidPath := getServicePIDPath(serviceName)
	info, err := os.Stat(pidPath)
	if err != nil {
		return ""
	}

	duration := time.Since(info.ModTime())
	if duration < time.Minute {
		return fmt.Sprintf("%ds", int(duration.Seconds()))
	} else if duration < time.Hour {
		return fmt.Sprintf("%dm", int(duration.Minutes()))
	} else if duration < 24*time.Hour {
		return fmt.Sprintf("%dh %dm", int(duration.Hours()), int(duration.Minutes())%60)
	}
	return fmt.Sprintf("%dd %dh", int(duration.Hours())/24, int(duration.Hours())%24)
}

// Command implementations

func runServicesStart(cmd *cobra.Command, args []string) error {
	config, err := loadServicesConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Ensure config file exists
	if _, err := os.Stat(getServicesConfigPath()); os.IsNotExist(err) {
		if err := saveServicesConfig(config); err != nil {
			return fmt.Errorf("failed to create config: %w", err)
		}
	}

	if len(args) > 0 {
		// Start specific service
		return startService(config, args[0])
	}

	// Start all enabled services
	var started, skipped, failed int
	for _, svc := range config.Services {
		if !svc.Enabled {
			skipped++
			continue
		}

		running, _ := isServiceRunning(svc.Name)
		if running {
			fmt.Printf("  %s already running\n", svc.Name)
			continue
		}

		if err := startService(config, svc.Name); err != nil {
			fmt.Printf("  %s failed: %v\n", svc.Name, err)
			failed++
		} else {
			started++
		}
	}

	fmt.Printf("\nStarted %d service(s)", started)
	if skipped > 0 {
		fmt.Printf(", %d disabled", skipped)
	}
	if failed > 0 {
		fmt.Printf(", %d failed", failed)
	}
	fmt.Println()

	return nil
}

func startService(config *ServicesConfig, name string) error {
	svc := findService(config, name)
	if svc == nil {
		return fmt.Errorf("unknown service: %s", name)
	}

	running, pid := isServiceRunning(name)
	if running {
		return fmt.Errorf("already running (PID %d)", pid)
	}

	// Ensure log directory exists
	logPath := getServiceLogPath(name)
	if err := os.MkdirAll(filepath.Dir(logPath), 0755); err != nil {
		return fmt.Errorf("failed to create log directory: %w", err)
	}

	// Open log file
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}

	// Start the process
	daemonCmd := exec.Command(svc.Command, svc.Args...)
	daemonCmd.Stdout = logFile
	daemonCmd.Stderr = logFile
	daemonCmd.Stdin = nil

	// Detach from parent process
	daemonCmd.SysProcAttr = &syscall.SysProcAttr{
		Setsid: true,
	}

	if err := daemonCmd.Start(); err != nil {
		logFile.Close()
		return fmt.Errorf("failed to start: %w", err)
	}

	// Write PID file
	if err := writeServicePID(name, daemonCmd.Process.Pid); err != nil {
		logFile.Close()
		return fmt.Errorf("failed to write PID: %w", err)
	}

	// Don't wait for process - it's a daemon
	go func() {
		daemonCmd.Wait()
		logFile.Close()
	}()

	fmt.Printf("  %s started (PID %d)\n", name, daemonCmd.Process.Pid)
	return nil
}

func runServicesStop(cmd *cobra.Command, args []string) error {
	config, err := loadServicesConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	if len(args) > 0 {
		// Stop specific service
		return stopService(args[0])
	}

	// Stop all running services
	var stopped int
	for _, svc := range config.Services {
		running, _ := isServiceRunning(svc.Name)
		if !running {
			continue
		}

		if err := stopService(svc.Name); err != nil {
			fmt.Printf("  %s failed to stop: %v\n", svc.Name, err)
		} else {
			stopped++
		}
	}

	fmt.Printf("\nStopped %d service(s)\n", stopped)
	return nil
}

func stopService(name string) error {
	running, pid := isServiceRunning(name)
	if !running {
		return fmt.Errorf("not running")
	}

	process, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("failed to find process: %w", err)
	}

	if err := process.Signal(syscall.SIGTERM); err != nil {
		return fmt.Errorf("failed to send signal: %w", err)
	}

	// Wait briefly for graceful shutdown
	for i := 0; i < 10; i++ {
		time.Sleep(100 * time.Millisecond)
		if err := process.Signal(syscall.Signal(0)); err != nil {
			break
		}
	}

	removeServicePID(name)
	fmt.Printf("  %s stopped\n", name)
	return nil
}

func runServicesStatus(cmd *cobra.Command, args []string) error {
	config, err := loadServicesConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	var statuses []ServiceStatus
	var problems []string

	for _, svc := range config.Services {
		running, pid := isServiceRunning(svc.Name)
		status := ServiceStatus{
			Name:        svc.Name,
			Running:     running,
			PID:         pid,
			Description: svc.Description,
			Enabled:     svc.Enabled,
		}

		if running {
			status.Uptime = getServiceUptime(svc.Name)
		}

		statuses = append(statuses, status)

		// Track problems: enabled but not running
		if svc.Enabled && !running {
			problems = append(problems, svc.Name)
		}
	}

	// Check-only mode: silent, just exit code
	if servicesCheckOnly {
		if len(problems) > 0 {
			os.Exit(1)
		}
		return nil
	}

	// Quiet mode: brief output with exit code
	if servicesQuiet {
		if len(problems) > 0 {
			fmt.Printf("Services not running: %s\n", strings.Join(problems, ", "))
			os.Exit(1)
		}
		return nil
	}

	// JSON output
	if servicesJSONOut {
		data, _ := json.MarshalIndent(statuses, "", "  ")
		fmt.Println(string(data))
		return nil
	}

	// Human-readable output
	fmt.Println("HAL 9000 Services Status")
	fmt.Println("========================")

	for _, s := range statuses {
		var statusStr string
		if s.Running {
			statusStr = fmt.Sprintf("\033[32m●\033[0m running  (pid %d, uptime %s)", s.PID, s.Uptime)
		} else if s.Enabled {
			statusStr = "\033[31m○\033[0m stopped"
		} else {
			statusStr = "\033[90m○\033[0m disabled"
		}
		fmt.Printf("  %-16s %s\n", s.Name, statusStr)
	}

	if len(problems) > 0 {
		fmt.Printf("\n\033[33mWarning:\033[0m %d enabled service(s) not running\n", len(problems))
	}

	return nil
}

func runServicesRestart(cmd *cobra.Command, args []string) error {
	config, err := loadServicesConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	if len(args) > 0 {
		// Restart specific service
		name := args[0]
		running, _ := isServiceRunning(name)
		if running {
			if err := stopService(name); err != nil {
				return fmt.Errorf("failed to stop %s: %w", name, err)
			}
			time.Sleep(500 * time.Millisecond)
		}
		return startService(config, name)
	}

	// Restart all enabled services
	for _, svc := range config.Services {
		if !svc.Enabled {
			continue
		}

		running, _ := isServiceRunning(svc.Name)
		if running {
			stopService(svc.Name)
			time.Sleep(500 * time.Millisecond)
		}
		startService(config, svc.Name)
	}

	return nil
}

func runServicesLogs(cmd *cobra.Command, args []string) error {
	config, err := loadServicesConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	var services []string
	if len(args) > 0 {
		services = args
	} else {
		for _, svc := range config.Services {
			services = append(services, svc.Name)
		}
	}

	for _, name := range services {
		logPath := getServiceLogPath(name)
		if _, err := os.Stat(logPath); os.IsNotExist(err) {
			continue
		}

		if len(services) > 1 {
			fmt.Printf("\n=== %s ===\n", name)
		}

		if err := tailFile(logPath, servicesLogTail); err != nil {
			fmt.Printf("Error reading %s logs: %v\n", name, err)
		}
	}

	return nil
}

func tailFile(path string, n int) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	if err := scanner.Err(); err != nil {
		return err
	}

	start := len(lines) - n
	if start < 0 {
		start = 0
	}

	for _, line := range lines[start:] {
		fmt.Println(line)
	}

	return nil
}

func runServicesDiagnose(cmd *cobra.Command, args []string) error {
	config, err := loadServicesConfig()
	if err != nil {
		fmt.Printf("\033[31mConfig Error:\033[0m %v\n", err)
		fmt.Println("\nTo fix: Check ~/.config/hal9000/services.yaml syntax")
		return nil
	}

	var services []string
	if len(args) > 0 {
		services = args
	} else {
		for _, svc := range config.Services {
			if svc.Enabled {
				services = append(services, svc.Name)
			}
		}
	}

	fmt.Println("HAL 9000 Service Diagnostics")
	fmt.Println("============================\n")

	hasProblems := false

	for _, name := range services {
		svc := findService(config, name)
		if svc == nil {
			fmt.Printf("\033[31m[%s]\033[0m Unknown service\n", name)
			fmt.Printf("  Available services: ")
			for i, s := range config.Services {
				if i > 0 {
					fmt.Print(", ")
				}
				fmt.Print(s.Name)
			}
			fmt.Println()
			hasProblems = true
			continue
		}

		fmt.Printf("[%s]\n", name)

		// Check if enabled
		if !svc.Enabled {
			fmt.Printf("  Status: \033[90mdisabled\033[0m\n")
			fmt.Printf("  To enable: Edit ~/.config/hal9000/services.yaml\n\n")
			continue
		}

		// Check executable
		execPath := svc.Command
		if !filepath.IsAbs(execPath) {
			// Try to find in PATH
			foundPath, err := exec.LookPath(execPath)
			if err != nil {
				fmt.Printf("  Executable: \033[31mNOT FOUND\033[0m (%s)\n", execPath)
				fmt.Printf("  Problem: Command not in PATH\n")
				fmt.Printf("  To fix: Either:\n")
				fmt.Printf("    1. Add directory containing '%s' to PATH\n", execPath)
				fmt.Printf("    2. Use absolute path in ~/.config/hal9000/services.yaml\n")
				hasProblems = true
				fmt.Println()
				continue
			}
			execPath = foundPath
		}

		// Check if file exists
		info, err := os.Stat(execPath)
		if os.IsNotExist(err) {
			fmt.Printf("  Executable: \033[31mNOT FOUND\033[0m (%s)\n", execPath)
			fmt.Printf("  Problem: File does not exist\n")
			fmt.Printf("  To fix: Update command path in ~/.config/hal9000/services.yaml\n")
			hasProblems = true
			fmt.Println()
			continue
		} else if err != nil {
			fmt.Printf("  Executable: \033[31mERROR\033[0m %v\n", err)
			hasProblems = true
			fmt.Println()
			continue
		}

		// Check if executable
		if info.Mode()&0111 == 0 {
			fmt.Printf("  Executable: \033[31mNOT EXECUTABLE\033[0m (%s)\n", execPath)
			fmt.Printf("  Problem: File is not executable\n")
			fmt.Printf("  To fix: chmod +x %s\n", execPath)
			hasProblems = true
			fmt.Println()
			continue
		}

		fmt.Printf("  Executable: \033[32mOK\033[0m (%s)\n", execPath)

		// Check process status
		running, pid := isServiceRunning(name)
		if running {
			uptime := getServiceUptime(name)
			fmt.Printf("  Process: \033[32mrunning\033[0m (PID %d, uptime %s)\n", pid, uptime)
		} else {
			fmt.Printf("  Process: \033[31mnot running\033[0m\n")
			hasProblems = true

			// Check for stale PID file
			pidPath := getServicePIDPath(name)
			if _, err := os.Stat(pidPath); err == nil {
				fmt.Printf("  Warning: Stale PID file exists at %s\n", pidPath)
				fmt.Printf("  To fix: rm %s\n", pidPath)
			}
		}

		// Check logs for recent errors
		logPath := getServiceLogPath(name)
		if _, err := os.Stat(logPath); err == nil {
			fmt.Printf("  Log file: %s\n", logPath)

			// Read last few lines looking for errors
			if lastLines, err := readLastLines(logPath, 20); err == nil {
				var errorLines []string
				for _, line := range lastLines {
					lineLower := strings.ToLower(line)
					if strings.Contains(lineLower, "error") ||
						strings.Contains(lineLower, "fatal") ||
						strings.Contains(lineLower, "panic") ||
						strings.Contains(lineLower, "failed") {
						errorLines = append(errorLines, line)
					}
				}
				if len(errorLines) > 0 {
					fmt.Printf("  Recent errors in log:\n")
					// Show at most 5 error lines
					start := 0
					if len(errorLines) > 5 {
						start = len(errorLines) - 5
					}
					for _, line := range errorLines[start:] {
						// Truncate long lines
						if len(line) > 100 {
							line = line[:97] + "..."
						}
						fmt.Printf("    \033[33m%s\033[0m\n", line)
					}
				}
			}
		} else {
			fmt.Printf("  Log file: none (service may not have run yet)\n")
		}

		fmt.Println()
	}

	// Summary and recommendations
	if hasProblems {
		fmt.Println("Recommendations")
		fmt.Println("---------------")
		fmt.Println("1. Fix any executable or configuration issues above")
		fmt.Println("2. Start services: hal9000 services start")
		fmt.Println("3. Check logs: hal9000 services logs")
	} else {
		fmt.Println("\033[32mAll services healthy.\033[0m")
	}

	return nil
}

func readLastLines(path string, n int) ([]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	start := len(lines) - n
	if start < 0 {
		start = 0
	}

	return lines[start:], nil
}
