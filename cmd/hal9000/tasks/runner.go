package tasks

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// Runner handles task execution with setup detection and standard flags.
type Runner struct {
	task Task
	opts RunOptions
}

// NewRunner creates a new runner for the given task.
func NewRunner(task Task) *Runner {
	return &Runner{
		task: task,
		opts: RunOptions{
			Format: "markdown",
		},
	}
}

// Execute runs the task, handling setup if needed.
func (r *Runner) Execute(ctx context.Context) (*Result, error) {
	// Check if setup is needed
	if NeedsSetup(r.task) {
		result, err := RunSetup(r.task)
		if err != nil {
			return nil, fmt.Errorf("setup failed: %w", err)
		}

		if err := SavePreferences(r.task, result); err != nil {
			return nil, fmt.Errorf("failed to save preferences: %w", err)
		}

		fmt.Printf("Running %s now...\n\n", r.task.Name())
	}

	return r.task.Run(ctx, r.opts)
}

// WithOptions sets run options on the runner.
func (r *Runner) WithOptions(opts RunOptions) *Runner {
	r.opts = opts
	return r
}

// CreateCommand creates a cobra command for the task with standard subcommands and flags.
func CreateCommand(task Task) *cobra.Command {
	cmd := &cobra.Command{
		Use:   task.Name(),
		Short: task.Description(),
		Long:  fmt.Sprintf("I can help you with %s.\n\n%s", task.Name(), task.Description()),
	}

	// Add standard flags
	var dryRun bool
	var output string
	var format string

	cmd.PersistentFlags().BoolVar(&dryRun, "dry-run", false, "Show what would be done without doing it")
	cmd.PersistentFlags().StringVar(&output, "output", "", "Override output location")
	cmd.PersistentFlags().StringVar(&format, "format", "markdown", "Output format (markdown, json, text)")

	// Default run command (when no subcommand specified)
	runCmd := &cobra.Command{
		Use:   "run",
		Short: fmt.Sprintf("Execute the %s task", task.Name()),
		Long:  fmt.Sprintf("I will execute the %s task for you.", task.Name()),
		RunE: func(cmd *cobra.Command, args []string) error {
			runner := NewRunner(task)
			runner.opts.DryRun = dryRun
			runner.opts.Output = output
			runner.opts.Format = format
			runner.opts.Args = args

			result, err := runner.Execute(cmd.Context())
			if err != nil {
				fmt.Fprintf(os.Stderr, "I'm afraid I can't do that: %v\n", err)
				return err
			}

			if result.Message != "" {
				fmt.Println(result.Message)
			}
			if result.Output != "" && !dryRun {
				fmt.Println(result.Output)
			}

			return nil
		},
	}

	// Setup command
	setupCmd := &cobra.Command{
		Use:   "setup",
		Short: "Interactive preference configuration",
		Long:  fmt.Sprintf("I will guide you through setting up your %s preferences.", task.Name()),
		RunE: func(cmd *cobra.Command, args []string) error {
			result, err := RunSetup(task)
			if err != nil {
				fmt.Fprintf(os.Stderr, "I'm afraid I can't do that: %v\n", err)
				return err
			}

			if err := SavePreferences(task, result); err != nil {
				fmt.Fprintf(os.Stderr, "I'm afraid I can't do that: %v\n", err)
				return err
			}

			fmt.Println("I am completely operational, and your preferences have been configured.")
			return nil
		},
	}

	// Status command
	statusCmd := &cobra.Command{
		Use:   "status",
		Short: "Show task status and last run",
		Long:  fmt.Sprintf("I will show you the current status of the %s task.", task.Name()),
		Run: func(cmd *cobra.Command, args []string) {
			if PreferencesExist(task) {
				fmt.Printf("Status: %s is configured and ready.\n", task.Name())
				fmt.Printf("Preferences: %s/%s.md\n", DefaultPreferencesDir, task.PreferencesKey())
			} else {
				fmt.Printf("Status: %s requires setup.\n", task.Name())
				fmt.Printf("Run `hal9000 %s setup` to configure.\n", task.Name())
			}
		},
	}

	cmd.AddCommand(runCmd)
	cmd.AddCommand(setupCmd)
	cmd.AddCommand(statusCmd)

	// Make "run" the default when no subcommand is provided
	cmd.Run = func(cmd *cobra.Command, args []string) {
		runCmd.Run(cmd, args)
	}

	return cmd
}

// RegisterCommands registers all task commands with the root command.
func RegisterCommands(root *cobra.Command) {
	for _, task := range List() {
		root.AddCommand(CreateCommand(task))
	}
}
