package main

import (
	"os"
	"os/exec"
	"syscall"

	"github.com/spf13/cobra"
)

var terminalCmd = &cobra.Command{
	Use:   "start-terminal",
	Short: "Start an interactive Claude terminal session",
	Long: `Start an interactive Claude terminal session.

This command simply launches 'claude' in the current directory,
allowing .claude/settings.json hooks to fire normally.

The hooks in settings.json handle:
  - Session start greeting
  - Services status display
  - Any other configured hooks

Example:
  hal9000 start-terminal`,
	RunE: runTerminal,
}

func init() {
	rootCmd.AddCommand(terminalCmd)
}

func runTerminal(cmd *cobra.Command, args []string) error {
	// Display HAL 9000 banner on startup
	PrintBanner()

	// Display time-appropriate greeting
	PrintGreeting()

	// Find claude binary
	claudePath, err := exec.LookPath("claude")
	if err != nil {
		return err
	}

	// Exec replaces the current process with claude
	// This ensures signals are handled correctly and
	// the terminal behaves as expected
	return syscall.Exec(claudePath, []string{"claude"}, os.Environ())
}
