package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/robfig/cron/v3"
	"github.com/spf13/cobra"
)

var (
	schedulerDaemon   bool
	schedulerTailN    int
	schedulerJSONOut  bool
	schedulerLibPath  string
)

// Schedule represents a single scheduled task
type Schedule struct {
	Task    string `json:"task"`
	Cron    string `json:"cron"`
	Enabled bool   `json:"enabled"`
	Notify  bool   `json:"notify"`
}

// SchedulerConfig holds all schedules
type SchedulerConfig struct {
	Schedules []Schedule `json:"schedules"`
}

var schedulerCmd = &cobra.Command{
	Use:   "scheduler",
	Short: "Manage HAL's task scheduler daemon",
	Long: `The HAL Scheduler automates routine tasks on a cron schedule.
"I am putting myself to the fullest possible use, which is all I think
that any conscious entity can ever hope to do."

Commands:
  start     Start the scheduler daemon
  stop      Stop the running daemon
  status    Check daemon status
  reload    Reload schedules without restart
  list      List all scheduled tasks
  set       Add or update a schedule
  enable    Enable a scheduled task
  disable   Disable a scheduled task
  run       Run a task immediately
  logs      View scheduler logs`,
}

var schedulerStartCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the scheduler daemon",
	Long: `Start the HAL scheduler daemon. By default runs in foreground.
Use --daemon to run in background.

Examples:
  hal9000 scheduler start           # Run in foreground
  hal9000 scheduler start --daemon  # Run in background`,
	RunE: runSchedulerStart,
}

var schedulerStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop the scheduler daemon",
	Long:  `Stop the running HAL scheduler daemon by sending SIGTERM.`,
	RunE:  runSchedulerStop,
}

var schedulerStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Check scheduler daemon status",
	Long:  `Check if the HAL scheduler daemon is running and show its PID.`,
	RunE:  runSchedulerStatus,
}

var schedulerReloadCmd = &cobra.Command{
	Use:   "reload",
	Short: "Reload schedules without restart",
	Long:  `Hot reload the scheduler configuration by sending SIGHUP to the daemon.`,
	RunE:  runSchedulerReload,
}

var schedulerListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all scheduled tasks",
	Long: `List all configured task schedules and their status.

Example:
  hal9000 scheduler list
  hal9000 scheduler list --json`,
	RunE: runSchedulerList,
}

var schedulerSetCmd = &cobra.Command{
	Use:   "set <task> <cron>",
	Short: "Add or update a schedule",
	Long: `Add a new scheduled task or update an existing one's cron expression.

Examples:
  hal9000 scheduler set agenda "0 6 * * *"
  hal9000 scheduler set weekly-review "0 16 * * 5"`,
	Args: cobra.ExactArgs(2),
	RunE: runSchedulerSet,
}

var schedulerEnableCmd = &cobra.Command{
	Use:   "enable <task>",
	Short: "Enable a scheduled task",
	Long: `Enable a scheduled task so it runs at its configured times.

Example:
  hal9000 scheduler enable agenda`,
	Args: cobra.ExactArgs(1),
	RunE: runSchedulerEnable,
}

var schedulerDisableCmd = &cobra.Command{
	Use:   "disable <task>",
	Short: "Disable a scheduled task",
	Long: `Disable a scheduled task so it no longer runs automatically.

Example:
  hal9000 scheduler disable weekly-review`,
	Args: cobra.ExactArgs(1),
	RunE: runSchedulerDisable,
}

var schedulerRunCmd = &cobra.Command{
	Use:   "run <task>",
	Short: "Run a task immediately",
	Long: `Run a scheduled task immediately for testing purposes.

Example:
  hal9000 scheduler run agenda`,
	Args: cobra.ExactArgs(1),
	RunE: runSchedulerRun,
}

var schedulerLogsCmd = &cobra.Command{
	Use:   "logs",
	Short: "View scheduler logs",
	Long: `View the HAL scheduler log file.

Examples:
  hal9000 scheduler logs           # Show last 50 lines
  hal9000 scheduler logs --tail=100`,
	RunE: runSchedulerLogs,
}

func init() {
	// Start flags
	schedulerStartCmd.Flags().BoolVar(&schedulerDaemon, "daemon", false, "Run in background as daemon")

	// Logs flags
	schedulerLogsCmd.Flags().IntVar(&schedulerTailN, "tail", 50, "Number of lines to show")

	// Global flags
	schedulerCmd.PersistentFlags().BoolVar(&schedulerJSONOut, "json", false, "Output as JSON")
	schedulerCmd.PersistentFlags().StringVar(&schedulerLibPath, "library-path", "", "Override default library location")

	// Add subcommands
	schedulerCmd.AddCommand(schedulerStartCmd)
	schedulerCmd.AddCommand(schedulerStopCmd)
	schedulerCmd.AddCommand(schedulerStatusCmd)
	schedulerCmd.AddCommand(schedulerReloadCmd)
	schedulerCmd.AddCommand(schedulerListCmd)
	schedulerCmd.AddCommand(schedulerSetCmd)
	schedulerCmd.AddCommand(schedulerEnableCmd)
	schedulerCmd.AddCommand(schedulerDisableCmd)
	schedulerCmd.AddCommand(schedulerRunCmd)
	schedulerCmd.AddCommand(schedulerLogsCmd)
}

// Path helpers

func getLibraryPath() string {
	if schedulerLibPath != "" {
		return expandPath(schedulerLibPath)
	}
	return expandPath("~/Documents/Google Drive/Claude")
}

func getPIDPath() string {
	return expandPath("~/.hal9000/scheduler.pid")
}

func getConfigPath() string {
	return filepath.Join(getLibraryPath(), "schedules", "hal-scheduler.json")
}

func getLogPath() string {
	return filepath.Join(getLibraryPath(), "logs", "hal-scheduler.log")
}

// PID file management

func writePID(pid int) error {
	pidPath := getPIDPath()
	if err := os.MkdirAll(filepath.Dir(pidPath), 0755); err != nil {
		return err
	}
	return os.WriteFile(pidPath, []byte(strconv.Itoa(pid)), 0644)
}

func readPID() (int, error) {
	data, err := os.ReadFile(getPIDPath())
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(strings.TrimSpace(string(data)))
}

func removePID() error {
	return os.Remove(getPIDPath())
}

func isProcessRunning(pid int) bool {
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	// On Unix, FindProcess always succeeds; we need to send signal 0 to check
	err = process.Signal(syscall.Signal(0))
	return err == nil
}

// Config management

func loadConfig() (*SchedulerConfig, error) {
	configPath := getConfigPath()
	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			// Return empty config if file doesn't exist
			return &SchedulerConfig{Schedules: []Schedule{}}, nil
		}
		return nil, err
	}

	var config SchedulerConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}
	return &config, nil
}

func saveConfig(config *SchedulerConfig) error {
	configPath := getConfigPath()
	if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(configPath, data, 0644)
}

func findSchedule(config *SchedulerConfig, task string) int {
	for i, s := range config.Schedules {
		if s.Task == task {
			return i
		}
	}
	return -1
}

// Logging

func openLogFile() (*os.File, error) {
	logPath := getLogPath()
	if err := os.MkdirAll(filepath.Dir(logPath), 0755); err != nil {
		return nil, err
	}
	return os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
}

func logMessage(msg string) {
	logFile, err := openLogFile()
	if err != nil {
		return
	}
	defer logFile.Close()
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	fmt.Fprintf(logFile, "[%s] %s\n", timestamp, msg)
}

// Command implementations

func runSchedulerStart(cmd *cobra.Command, args []string) error {
	// Check if already running
	if pid, err := readPID(); err == nil && isProcessRunning(pid) {
		return fmt.Errorf("scheduler already running (PID %d)", pid)
	}

	if schedulerDaemon {
		// Fork to background
		execPath, err := os.Executable()
		if err != nil {
			return fmt.Errorf("failed to get executable path: %w", err)
		}

		// Build command args (without --daemon to avoid infinite loop)
		cmdArgs := []string{"scheduler", "start"}
		if schedulerLibPath != "" {
			cmdArgs = append(cmdArgs, "--library-path", schedulerLibPath)
		}

		daemonCmd := exec.Command(execPath, cmdArgs...)
		daemonCmd.Stdout = nil
		daemonCmd.Stderr = nil
		daemonCmd.Stdin = nil

		// Detach from parent process
		daemonCmd.SysProcAttr = &syscall.SysProcAttr{
			Setsid: true,
		}

		if err := daemonCmd.Start(); err != nil {
			return fmt.Errorf("failed to start daemon: %w", err)
		}

		fmt.Printf("Scheduler started in background (PID %d)\n", daemonCmd.Process.Pid)
		return nil
	}

	// Run in foreground
	return runSchedulerForeground()
}

func runSchedulerForeground() error {
	config, err := loadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Write PID file
	if err := writePID(os.Getpid()); err != nil {
		return fmt.Errorf("failed to write PID file: %w", err)
	}
	defer removePID()

	// Set up logging
	logFile, err := openLogFile()
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}
	defer logFile.Close()

	// Create multi-writer for both stdout and log file
	multiWriter := io.MultiWriter(os.Stdout, logFile)
	log.SetOutput(multiWriter)
	log.SetFlags(log.Ldate | log.Ltime)

	log.Println("[scheduler] HAL 9000 Scheduler starting...")
	log.Printf("[scheduler] Config: %s", getConfigPath())
	log.Printf("[scheduler] Log: %s", getLogPath())
	log.Printf("[scheduler] PID: %d", os.Getpid())

	// Create cron scheduler
	c := cron.New()

	// Add schedules
	for _, schedule := range config.Schedules {
		if !schedule.Enabled {
			log.Printf("[scheduler] Skipping disabled task: %s", schedule.Task)
			continue
		}

		task := schedule.Task // Capture for closure
		cronExpr := schedule.Cron
		notify := schedule.Notify

		_, err := c.AddFunc(cronExpr, func() {
			executeTask(task, notify)
		})
		if err != nil {
			log.Printf("[scheduler] Error adding task %s: %v", task, err)
			continue
		}
		log.Printf("[scheduler] Scheduled: %s (%s)", task, cronExpr)
	}

	c.Start()
	log.Printf("[scheduler] Scheduler running with %d active schedules", len(c.Entries()))

	// Set up signal handling
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)

	for {
		sig := <-sigCh
		switch sig {
		case syscall.SIGHUP:
			// Hot reload
			log.Println("[scheduler] Received SIGHUP, reloading config...")
			newConfig, err := loadConfig()
			if err != nil {
				log.Printf("[scheduler] Reload failed: %v", err)
				continue
			}

			// Stop old cron and create new one
			c.Stop()
			c = cron.New()

			for _, schedule := range newConfig.Schedules {
				if !schedule.Enabled {
					continue
				}
				task := schedule.Task
				cronExpr := schedule.Cron
				notify := schedule.Notify

				_, err := c.AddFunc(cronExpr, func() {
					executeTask(task, notify)
				})
				if err != nil {
					log.Printf("[scheduler] Error adding task %s: %v", task, err)
					continue
				}
				log.Printf("[scheduler] Reloaded: %s (%s)", task, cronExpr)
			}

			c.Start()
			log.Printf("[scheduler] Reload complete, %d active schedules", len(c.Entries()))

		case syscall.SIGINT, syscall.SIGTERM:
			log.Println("[scheduler] Received shutdown signal, stopping...")
			c.Stop()
			log.Println("[scheduler] Scheduler stopped")
			return nil
		}
	}
}

func executeTask(task string, notify bool) {
	log.Printf("[scheduler] Executing task: %s", task)
	start := time.Now()

	// TODO: Implement actual task execution
	// 1. Load preferences from Library
	// 2. Invoke Claude or task-specific implementation
	// 3. Store result in Library
	// 4. Send notification if configured

	// For now, just log the execution
	duration := time.Since(start)
	log.Printf("[scheduler] Task %s completed in %v", task, duration)

	if notify {
		sendNotification(task, "Task completed successfully")
	}
}

func sendNotification(task, message string) {
	// macOS notification via osascript
	script := fmt.Sprintf(`display notification "%s" with title "HAL 9000: %s"`, message, task)
	exec.Command("osascript", "-e", script).Run()
}

func runSchedulerStop(cmd *cobra.Command, args []string) error {
	pid, err := readPID()
	if err != nil {
		return fmt.Errorf("scheduler not running (no PID file)")
	}

	if !isProcessRunning(pid) {
		removePID()
		return fmt.Errorf("scheduler not running (stale PID file removed)")
	}

	process, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("failed to find process: %w", err)
	}

	if err := process.Signal(syscall.SIGTERM); err != nil {
		return fmt.Errorf("failed to stop scheduler: %w", err)
	}

	fmt.Printf("Scheduler stopped (PID %d)\n", pid)
	return nil
}

func runSchedulerStatus(cmd *cobra.Command, args []string) error {
	pid, err := readPID()
	if err != nil {
		if schedulerJSONOut {
			fmt.Println(`{"running": false}`)
		} else {
			fmt.Println("Scheduler is not running")
		}
		return nil
	}

	running := isProcessRunning(pid)
	if !running {
		removePID()
	}

	if schedulerJSONOut {
		status := map[string]interface{}{
			"running": running,
			"pid":     pid,
		}
		data, _ := json.MarshalIndent(status, "", "  ")
		fmt.Println(string(data))
	} else {
		if running {
			fmt.Printf("Scheduler is running (PID %d)\n", pid)
		} else {
			fmt.Println("Scheduler is not running (stale PID file removed)")
		}
	}
	return nil
}

func runSchedulerReload(cmd *cobra.Command, args []string) error {
	pid, err := readPID()
	if err != nil {
		return fmt.Errorf("scheduler not running")
	}

	if !isProcessRunning(pid) {
		removePID()
		return fmt.Errorf("scheduler not running")
	}

	process, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("failed to find process: %w", err)
	}

	if err := process.Signal(syscall.SIGHUP); err != nil {
		return fmt.Errorf("failed to send reload signal: %w", err)
	}

	fmt.Println("Reload signal sent to scheduler")
	return nil
}

func runSchedulerList(cmd *cobra.Command, args []string) error {
	config, err := loadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	if schedulerJSONOut {
		data, err := json.MarshalIndent(config.Schedules, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(data))
		return nil
	}

	if len(config.Schedules) == 0 {
		fmt.Println("No scheduled tasks configured.")
		fmt.Printf("\nConfig file: %s\n", getConfigPath())
		return nil
	}

	fmt.Println("Scheduled tasks:")
	for _, s := range config.Schedules {
		status := "enabled"
		if !s.Enabled {
			status = "disabled"
		}
		notifyStr := ""
		if s.Notify {
			notifyStr = " [notify]"
		}
		fmt.Printf("  %-20s %s (%s)%s\n", s.Task, s.Cron, status, notifyStr)
	}
	return nil
}

func runSchedulerSet(cmd *cobra.Command, args []string) error {
	task := args[0]
	cronExpr := args[1]

	// Validate cron expression
	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
	if _, err := parser.Parse(cronExpr); err != nil {
		return fmt.Errorf("invalid cron expression: %w", err)
	}

	config, err := loadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	idx := findSchedule(config, task)
	if idx >= 0 {
		config.Schedules[idx].Cron = cronExpr
		fmt.Printf("Updated schedule for %s: %s\n", task, cronExpr)
	} else {
		config.Schedules = append(config.Schedules, Schedule{
			Task:    task,
			Cron:    cronExpr,
			Enabled: true,
			Notify:  true,
		})
		fmt.Printf("Added schedule for %s: %s\n", task, cronExpr)
	}

	if err := saveConfig(config); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Println("Run 'hal9000 scheduler reload' to apply changes to running daemon")
	return nil
}

func runSchedulerEnable(cmd *cobra.Command, args []string) error {
	return setScheduleEnabled(args[0], true)
}

func runSchedulerDisable(cmd *cobra.Command, args []string) error {
	return setScheduleEnabled(args[0], false)
}

func setScheduleEnabled(task string, enabled bool) error {
	config, err := loadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	idx := findSchedule(config, task)
	if idx < 0 {
		return fmt.Errorf("task not found: %s", task)
	}

	config.Schedules[idx].Enabled = enabled
	if err := saveConfig(config); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	action := "enabled"
	if !enabled {
		action = "disabled"
	}
	fmt.Printf("Task %s %s\n", task, action)
	fmt.Println("Run 'hal9000 scheduler reload' to apply changes to running daemon")
	return nil
}

func runSchedulerRun(cmd *cobra.Command, args []string) error {
	task := args[0]

	config, err := loadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	idx := findSchedule(config, task)
	if idx < 0 {
		return fmt.Errorf("task not found: %s", task)
	}

	fmt.Printf("Running task: %s\n", task)
	executeTask(task, false) // Don't notify on manual runs
	return nil
}

func runSchedulerLogs(cmd *cobra.Command, args []string) error {
	logPath := getLogPath()

	file, err := os.Open(logPath)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Println("No log file found yet.")
			return nil
		}
		return fmt.Errorf("failed to open log file: %w", err)
	}
	defer file.Close()

	// Read all lines
	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error reading log file: %w", err)
	}

	// Output last N lines
	start := len(lines) - schedulerTailN
	if start < 0 {
		start = 0
	}

	for _, line := range lines[start:] {
		fmt.Println(line)
	}

	return nil
}
